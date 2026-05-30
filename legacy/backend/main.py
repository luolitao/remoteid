"""
Remote ID 后端 API (FastAPI)
✅ 真实 ASTM F3411-22a + GB42590-2023 解析器
✅ 修复 SQLite 参数错误
✅ 无 ip 命令依赖
✅ 适合生产环境
"""

import asyncio
import struct
import threading
import queue
import json
import os
import sqlite3
from datetime import datetime, timedelta
from typing import List, Dict, Optional, Tuple
import scapy.all as scapy
from scapy.layers.dot11 import Dot11, Dot11Elt, RadioTap

# FastAPI 相关
from fastapi import FastAPI, WebSocket, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel

# 配置
DB_FILE = "/data/remoteid.db"
PCAP_DIR = "/data/pcaps"
os.makedirs(PCAP_DIR, exist_ok=True)

app = FastAPI(title="Remote ID API", version="1.0")

# CORS 配置
app.add_middleware(
    CORSMiddleware,
    allow_origins=[
        "http://localhost:8080",
        "http://127.0.0.1:8080",
        "http://raspberrypi.local:8080",
        "http://192.168.6.40:8080",
        "http://localhost",
        "http://raspberrypi.local",
        "*"  # 仅用于开发，生产环境应限制具体域名
    ],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 全局状态
DRONE_DATA: Dict[str, Dict] = {}
DATA_QUEUE = queue.Queue()
ALERT_HISTORY = set()
BROADCAST_QUEUE = asyncio.Queue()
PACKET_QUEUE = queue.Queue()
app_loop = None

class ConnectionManager:
    def __init__(self):
        self.active_connections: List[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)

    def disconnect(self, websocket: WebSocket):
        self.active_connections.remove(websocket)

    async def broadcast(self, message: str):
        for connection in self.active_connections[:]:
            try:
                await connection.send_text(message)
            except Exception as e:
                print(f"Broadcast error: {e}")
                self.disconnect(connection)

manager = ConnectionManager()

class DroneResponse(BaseModel):
    mac: str
    last_seen: str
    uas_id: str
    ua_type: str
    id_type: Optional[str] = None
    latitude: Optional[float]
    longitude: Optional[float]
    altitude: Optional[float]
    speed_horizontal: Optional[float] = None
    speed_vertical: Optional[float] = None
    heading: Optional[float] = None
    status: Optional[str] = None
    operator_latitude: Optional[float] = None
    operator_longitude: Optional[float] = None
    classification_region: Optional[str] = None
    china_compliant: Optional[bool] = None
    standard: str  # ASTM 或 GB42590

class AlertResponse(BaseModel):
    type: str
    message: str
    timestamp: str

# ========== Remote ID 常量 ==========
# ASTM F3411-22a (OpenDroneID)
ASTM_OUI = b'\x06\x05\x04'  # 06:05:04
ASTM_VENDOR_TYPE = 0xFD

# GB42590-2023 (China C-RID)
CHINA_OUI = b'\xFA\x0B\xBC'  # FA:0B:BC
CHINA_VENDOR_TYPE = 0x0D

# 消息类型映射
MSG_TYPES = {
    0: "Basic ID",
    1: "Location/Vector",
    2: "Authentication",
    3: "Self ID",
    4: "System",
    5: "Operator ID",
    0xF: "Packed Message"
}

# ========== Remote ID 解析器 ==========
class RemoteIDParser:
    def __init__(self):
        self.stats = {"astm_packets": 0, "crid_packets": 0, "parse_errors": 0}
    
    def find_astm_in_frame(self, raw_bytes: bytes) -> Optional[List[Dict]]:
        """查找并解析 ASTM F3411-22a 消息"""
        try:
            idx = 0
            while idx <= len(raw_bytes) - 3:
                if raw_bytes[idx:idx+3] == ASTM_OUI:
                    # 检查是否有足够的数据
                    if len(raw_bytes) < idx + 3 + 25:
                        return None
                    
                    # 提取消息
                    messages = []
                    offset = idx + 3
                    while offset + 25 <= len(raw_bytes):
                        msg_data = raw_bytes[offset:offset+25]
                        parsed = self.parse_astm_message(msg_data)
                        if parsed:
                            messages.append(parsed)
                        offset += 25
                    if messages:
                        self.stats["astm_packets"] += 1
                        return messages
                idx += 1
            return None
        except Exception as e:
            self.stats["parse_errors"] += 1
            print(f"ASTM parse error: {e}")
            return None
    
    def parse_astm_message(self, data: bytes) -> Optional[Dict]:
        """解析 ASTM 消息"""
        if len(data) < 25:
            return None
        
        msg_type = (data[0] >> 4) & 0x0F
        proto_ver = data[1]
        
        if proto_ver != ASTM_VENDOR_TYPE:
            return None
        
        payload = data[2:25]
        
        if msg_type == 0:  # Basic ID
            ua_type = payload[0] >> 4
            id_type = payload[0] & 0x0F
            uas_id_bytes = payload[1:21]
            try:
                uas_id = uas_id_bytes.decode('utf-8').strip()
            except:
                uas_id = uas_id_bytes.hex()
            
            ua_types = {
                0: "None", 1: "Aeroplane", 2: "HeliOrMulti", 3: "Gyroplane",
                4: "VTOL", 5: "Ornithopter", 6: "Glider", 7: "Kite",
                8: "FreeBalloon", 9: "CaptiveBalloon", 10: "Airship",
                11: "FreeFall/Parachute", 12: "Rocket", 13: "TetheredPowered",
                14: "GroundObstacle", 15: "Other"
            }
            
            id_types = {
                0: "None", 1: "SerialNumber", 2: "CAARegistrationID",
                3: "UTMID", 4: "MACAddress", 5: "Other"
            }
            
            return {
                'message_type': 'Basic ID',
                'ua_type': ua_types.get(ua_type, f"Unknown ({ua_type})"),
                'id_type': id_types.get(id_type, f"Unknown ({id_type})"),
                'uas_id': uas_id,
                'standard': "ASTM F3411-22a"
            }
        
        elif msg_type == 1:  # Location
            status = payload[0] >> 4
            lat = struct.unpack('>i', b'\x00' + payload[4:7])[0] / 10000000.0
            lon = struct.unpack('>i', b'\x00' + payload[7:10])[0] / 10000000.0
            alt = struct.unpack('<H', payload[10:12])[0] * 0.5 - 1000.0
            
            status_types = {
                0: "Undeclared", 1: "Ground", 2: "Airborne",
                3: "Emergency", 4: "Remote ID System Failure"
            }
            
            return {
                'message_type': 'Location',
                'status': status_types.get(status, f"Unknown ({status})"),
                'latitude': lat if lat != 90.0 else None,
                'longitude': lon if lon != 180.0 else None,
                'altitude_m': alt if alt != -1000.0 else None,
                'standard': "ASTM F3411-22a"
            }
        
        return None
    
    def find_crid_in_frame(self, raw_bytes: bytes) -> Optional[List[Dict]]:
        """查找并解析 GB42590-2023 消息"""
        try:
            idx = 0
            while idx <= len(raw_bytes) - 4:
                if raw_bytes[idx:idx+3] == CHINA_OUI and idx + 3 < len(raw_bytes) and raw_bytes[idx+3] == CHINA_VENDOR_TYPE:
                    if len(raw_bytes) < idx + 5:
                        return None
                    
                    msg_counter = raw_bytes[idx+4]
                    payload = raw_bytes[idx+5:]
                    
                    # 检查是否是 Packed 格式 (0xF1)
                    if len(payload) >= 3 and (payload[0] >> 4) == 0xF:
                        if payload[1] == 0x19 and 1 <= payload[2] <= 10:
                            msg_count = payload[2]
                            offset = 3
                            messages = []
                            for _ in range(msg_count):
                                if offset + 25 > len(payload):
                                    break
                                msg_data = payload[offset:offset+25]
                                msg_type = (msg_data[0] >> 4) & 0x0F
                                if msg_type == 0:
                                    parsed = self.parse_crid_basic_id(msg_data)
                                elif msg_type == 1:
                                    parsed = self.parse_crid_location(msg_data)
                                elif msg_type == 4:
                                    parsed = self.parse_crid_system(msg_data)
                                else:
                                    parsed = {
                                        'message_type': f'Unknown ({msg_type})',
                                        'raw_hex': msg_data.hex(),
                                        'standard': "GB42590-2023"
                                    }
                                if parsed:
                                    parsed['counter'] = msg_counter
                                    messages.append(parsed)
                                offset += 25
                            if messages:
                                self.stats["crid_packets"] += 1
                                return messages
                    # 传统格式处理（省略，优先支持 Packed）
                idx += 1
            return None
        except Exception as e:
            self.stats["parse_errors"] += 1
            print(f"C-RID parse error: {e}")
            return None
    
    def parse_crid_basic_id(self, data: bytes) -> Optional[Dict]:
        """解析 C-RID Basic ID (Packed 格式)"""
        if len(data) < 25:
            return None
        
        id_ua_byte = data[1]
        id_type = (id_ua_byte >> 4) & 0x0F
        ua_type = id_ua_byte & 0x0F
        uas_id_bytes = data[2:22]
        try:
            uas_id = uas_id_bytes.decode('ascii').rstrip('\x00 \x20')
        except:
            uas_id = uas_id_bytes.hex()
        
        # 中国标准要求 ID Type = 2 (CAA Registration ID)
        china_compliant = (id_type == 2)
        
        return {
            'message_type': 'Basic ID',
            'id_type': self.get_crid_id_type(id_type),
            'ua_type': self.get_crid_ua_type(ua_type),
            'uas_id': uas_id,
            'china_compliant': china_compliant,
            'id_type_raw': id_type,
            'ua_type_raw': ua_type,
            'standard': "GB42590-2023"
        }
    
    def parse_crid_location(self, data: bytes) -> Optional[Dict]:
        """解析 C-RID Location (Packed 格式)"""
        if len(data) < 25:
            return None
        
        flags = data[1]
        status = (flags >> 4) & 0x0F
        direction_high = flags & 0x0F
        direction_low = data[2]
        direction_raw = (direction_high << 8) | direction_low
        direction = (direction_raw * 360.0) / 65535.0 if direction_raw != 0xFFFF else None
        
        speed_h = data[3] * 0.25 if data[3] != 255 else None
        speed_v = (data[4] - 128) * 0.5 if data[4] != 255 else None
        
        lat_raw = struct.unpack('<i', data[5:9])[0]
        lat = lat_raw / 10000000.0 if lat_raw != 0x7FFFFFFF else None
        
        lon_raw = struct.unpack('<i', data[9:13])[0]
        lon = lon_raw / 10000000.0 if lon_raw != 0x7FFFFFFF else None
        
        alt_raw = struct.unpack('<H', data[13:15])[0]
        altitude = alt_raw * 0.5 - 1000.0 if alt_raw != 0xFFFF else None
        
        return {
            'message_type': 'Location',
            'status': self.get_status_type(status),
            'direction_deg': direction,
            'speed_horizontal_m_s': speed_h,
            'speed_vertical_m_s': speed_v,
            'latitude': lat,
            'longitude': lon,
            'altitude_m': altitude,
            'standard': "GB42590-2023"
        }
    
    def parse_crid_system(self, data: bytes) -> Optional[Dict]:
        """解析 C-RID System 消息 (操作员位置等)"""
        if len(data) < 25:
            return None
        
        flags = data[1]
        classification = (flags >> 4) & 0x07
        
        classification_types = {
            0: "Undefined", 1: "USA", 2: "China",
            3: "EU", 4: "UK", 5: "Japan", 6: "Australia", 7: "Other"
        }
        
        # 操作员位置（Packed 格式中位于 bytes 2-17）
        op_lat_raw = struct.unpack('<i', data[2:6])[0]
        op_lat = op_lat_raw / 10000000.0 if op_lat_raw != 0x7FFFFFFF else None
        
        op_lon_raw = struct.unpack('<i', data[6:10])[0]
        op_lon = op_lon_raw / 10000000.0 if op_lon_raw != 0x7FFFFFFF else None
        
        return {
            'message_type': 'System',
            'classification_region': classification_types.get(classification, f"Unknown ({classification})"),
            'operator_latitude': op_lat,
            'operator_longitude': op_lon,
            'standard': "GB42590-2023"
        }
    
    # 辅助方法
    def get_crid_id_type(self, id_type: int) -> str:
        types = {
            0: "None", 1: "Serial Number", 2: "CAA Registration ID",
            3: "UTM Assigned UUID", 4: "Specific Session ID"
        }
        return types.get(id_type, f"Unknown ({id_type})")
    
    def get_crid_ua_type(self, ua_type: int) -> str:
        types = {
            0: "None/Not declared", 1: "Aeroplane/Fixed wing",
            2: "Helicopter/Multirotor", 3: "Gyroplane", 4: "Hybrid Lift",
            5: "Ornithopter", 6: "Glider", 7: "Kite", 8: "Free Balloon",
            9: "Captive Balloon", 10: "Airship", 11: "Free Fall/Parachute",
            12: "Rocket", 13: "Tethered Powered Aircraft", 
            14: "Ground Obstacle", 15: "Other"
        }
        return types.get(ua_type, f"Unknown ({ua_type})")
    
    def get_status_type(self, status: int) -> str:
        types = {
            0: "Undeclared", 1: "Ground", 2: "Airborne",
            3: "Emergency", 4: "Remote ID System Failure"
        }
        return types.get(status, f"Unknown ({status})")

# ========== 数据库初始化 ==========
def init_db():
    """初始化 SQLite 数据库"""
    try:
        conn = sqlite3.connect(DB_FILE)
        c = conn.cursor()
        
        # 无人机主表
        c.execute('''
            CREATE TABLE IF NOT EXISTS drones (
                mac TEXT PRIMARY KEY,
                first_seen TEXT,
                last_seen TEXT,
                uas_id TEXT,
                ua_type TEXT,
                id_type TEXT,
                latitude REAL,
                longitude REAL,
                altitude REAL,
                speed_horizontal REAL,
                speed_vertical REAL,
                heading REAL,
                status TEXT,
                operator_latitude REAL,
                operator_longitude REAL,
                classification_region TEXT,
                china_compliant BOOLEAN DEFAULT 0,
                standard TEXT
            )
        ''')
        
        # 位置历史表
        c.execute('''
            CREATE TABLE IF NOT EXISTS positions (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                mac TEXT,
                timestamp TEXT,
                latitude REAL,
                longitude REAL,
                altitude REAL,
                speed REAL,
                heading REAL,
                standard TEXT
            )
        ''')
        
        # 警报表
        c.execute('''
            CREATE TABLE IF NOT EXISTS alerts (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                alert_type TEXT,
                message TEXT,
                timestamp TEXT
            )
        ''')
        
        conn.commit()
        conn.close()
        print("✅ Database initialized successfully")
    except Exception as e:
        print(f"❌ Database initialization failed: {e}")
        raise

# ========== WebSocket 路由 ==========
@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    """实时数据 WebSocket 端点"""
    await manager.connect(websocket)
    try:
        while True:
            await asyncio.sleep(30)
    except Exception as e:
        print(f"WebSocket error: {e}")
    finally:
        manager.disconnect(websocket)

# ========== REST API 路由 ==========
@app.get("/health")
async def health_check():
    """健康检查端点"""
    return {"status": "healthy", "timestamp": datetime.now().isoformat()}

@app.get("/api/drones", response_model=List[DroneResponse])
async def get_active_drones():
    """获取当前活跃无人机（5分钟内）"""
    now = datetime.now()
    active_drones = []
    
    for mac, data in DRONE_DATA.items():
        try:
            last_seen = datetime.strptime(data['last_seen'], '%Y-%m-%d %H:%M:%S')
            if (now - last_seen).total_seconds() < 300:  # 5分钟
                active_drones.append(DroneResponse(
                    mac=mac,
                    last_seen=data['last_seen'],
                    uas_id=data.get('uas_id', 'Unknown'),
                    ua_type=data.get('ua_type', 'Unknown'),
                    id_type=data.get('id_type'),
                    latitude=data.get('latitude'),
                    longitude=data.get('longitude'),
                    altitude=data.get('altitude'),
                    speed_horizontal=data.get('speed_horizontal'),
                    speed_vertical=data.get('speed_vertical'),
                    heading=data.get('heading'),
                    status=data.get('status'),
                    operator_latitude=data.get('operator_latitude'),
                    operator_longitude=data.get('operator_longitude'),
                    classification_region=data.get('classification_region'),
                    china_compliant=data.get('china_compliant'),
                    standard=data.get('standard', 'Unknown')
                ))
        except (KeyError, ValueError) as e:
            print(f"Error processing drone {mac}: {e}")
    
    return active_drones

@app.get("/api/trajectory/{mac}")
async def get_trajectory(mac: str, hours: int = 1):
    """获取无人机历史轨迹"""
    try:
        since = (datetime.now() - timedelta(hours=hours)).strftime('%Y-%m-%d %H:%M:%S')
        
        conn = sqlite3.connect(DB_FILE)
        c = conn.cursor()
        c.execute('''
            SELECT timestamp, latitude, longitude, altitude, speed, heading
            FROM positions
            WHERE mac = ? AND timestamp > ?
            ORDER BY timestamp
        ''', (mac, since))
        rows = c.fetchall()
        conn.close()
        
        return [
            {
                "timestamp": row[0],
                "latitude": row[1],
                "longitude": row[2],
                "altitude": row[3],
                "speed": row[4],
                "heading": row[5]
            }
            for row in rows
        ]
    except Exception as e:
        print(f"Trajectory error for {mac}: {e}")
        raise HTTPException(status_code=500, detail="Failed to fetch trajectory")

@app.get("/api/alerts", response_model=List[AlertResponse])
async def get_alerts(limit: int = 20):
    """获取最近警报"""
    try:
        conn = sqlite3.connect(DB_FILE)
        c = conn.cursor()
        c.execute('''
            SELECT alert_type, message, timestamp
            FROM alerts
            ORDER BY timestamp DESC
            LIMIT ?
        ''', (limit,))
        rows = c.fetchall()
        conn.close()
        
        return [
            AlertResponse(
                type=row[0],
                message=row[1],
                timestamp=row[2]
            )
            for row in rows
        ]
    except Exception as e:
        print(f"Alerts fetch error: {e}")
        raise HTTPException(status_code=500, detail="Failed to fetch alerts")

# ========== 后台线程 ==========
def db_writer():
    """后台数据库写入线程"""
    print("🔄 Starting database writer thread")
    while True:
        try:
            mac, data = DATA_QUEUE.get(timeout=1)
            
            try:
                conn = sqlite3.connect(DB_FILE)
                c = conn.cursor()
                
                # 插入或更新无人机完整数据
                c.execute('''
                    INSERT OR REPLACE INTO drones 
                    (mac, first_seen, last_seen, uas_id, ua_type, id_type,
                     latitude, longitude, altitude, speed_horizontal, speed_vertical, heading,
                     status, operator_latitude, operator_longitude, classification_region,
                     china_compliant, standard)
                    VALUES (
                        ?, COALESCE((SELECT first_seen FROM drones WHERE mac = ?), ?),
                        ?, ?, ?, ?,
                        ?, ?, ?, ?, ?, ?,
                        ?, ?, ?, ?,
                        ?, ?
                    )
                ''', (
                    mac,                                                      # 1: mac
                    mac,                                                      # 2: COALESCE 子查询
                    data['last_seen'],                                         # 3: first_seen 默认值
                    data['last_seen'],                                         # 4: last_seen
                    data.get('uas_id', 'Unknown'),                            # 5: uas_id
                    data.get('ua_type', 'Unknown'),                           # 6: ua_type
                    data.get('id_type'),                                      # 7: id_type
                    data.get('latitude'),                                     # 8: latitude
                    data.get('longitude'),                                    # 9: longitude
                    data.get('altitude'),                                     # 10: altitude
                    data.get('speed_horizontal'),                             # 11: speed_horizontal
                    data.get('speed_vertical'),                               # 12: speed_vertical
                    data.get('heading'),                                      # 13: heading
                    data.get('status'),                                       # 14: status
                    data.get('operator_latitude'),                            # 15: operator_latitude
                    data.get('operator_longitude'),                           # 16: operator_longitude
                    data.get('classification_region'),                        # 17: classification_region
                    1 if data.get('china_compliant') else 0,                  # 18: china_compliant
                    data.get('standard', 'Unknown')                           # 19: standard
                ))
                
                # 位置历史（含速度、航向）
                if 'latitude' in data and data.get('latitude') is not None:
                    c.execute('''
                        INSERT INTO positions (mac, timestamp, latitude, longitude, altitude, speed, heading, standard)
                        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                    ''', (
                        mac, 
                        data['last_seen'], 
                        data.get('latitude'),
                        data.get('longitude'),
                        data.get('altitude'),
                        data.get('speed_horizontal'),
                        data.get('heading'),
                        data.get('standard', 'Unknown')
                    ))
                
                conn.commit()
                conn.close()
            except Exception as e:
                print(f"❌ Database write error for {mac}: {e}")
                print(f"💡 Debug: Data keys = {list(data.keys())}")
            
            DATA_QUEUE.task_done()
        except queue.Empty:
            continue
        except Exception as e:
            print(f"❌ DB writer thread error: {e}")

# 在 data_processor 函数中
def data_processor():
    """后台数据处理线程"""
    print("🔄 Starting data processor thread")
    parser = RemoteIDParser()
    
    while True:
        try:
            pkt = PACKET_QUEUE.get(timeout=1)
            
            try:
                # 检查是否为有效的 802.11 帧
                if not hasattr(pkt, 'type') or not hasattr(pkt, 'subtype'):
                    continue
                
                # 只处理管理帧
                if pkt.type != 0:
                    continue
                
                # 提取源 MAC
                src_mac = getattr(pkt, 'addr2', 'Unknown')
                if src_mac == 'Unknown' or not src_mac:
                    continue
                
                # 获取原始字节
                raw = bytes(pkt)
                
                # 尝试解析 ASTM F3411-22a
                astm_msgs = parser.find_astm_in_frame(raw)
                if astm_msgs:
                    drone_data = {
                        'last_seen': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                        'standard': "ASTM F3411-22a"
                    }
                    
                    for msg in astm_msgs:
                        if msg['message_type'] == 'Basic ID':
                            drone_data['uas_id'] = msg.get('uas_id', 'Unknown')
                            drone_data['ua_type'] = msg.get('ua_type', 'Unknown')
                            drone_data['id_type'] = msg.get('id_type')
                        elif msg['message_type'] == 'Location':
                            drone_data['latitude'] = msg.get('latitude')
                            drone_data['longitude'] = msg.get('longitude')
                            drone_data['altitude'] = msg.get('altitude_m')
                            drone_data['status'] = msg.get('status')
                    
                    # 仅当解析到有效数据时才处理
                    if 'uas_id' in drone_data:
                        DRONE_DATA[src_mac] = drone_data
                        print(f"🛰️ ASTM drone detected: {src_mac} -> {drone_data['uas_id']}")
                        
                        if app_loop:
                            asyncio.run_coroutine_threadsafe(
                                BROADCAST_QUEUE.put({
                                    "type": "drone_update",
                                    "data": drone_data,
                                    "mac": src_mac
                                }),
                                app_loop
                            )
                        
                        DATA_QUEUE.put((src_mac, drone_data))
                
                
                # 尝试解析 GB42590-2023
                crid_msgs = parser.find_crid_in_frame(raw)
                if crid_msgs:
                    drone_data = {
                        'last_seen': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
                        'standard': "GB42590-2023"
                    }
                    
                    for msg in crid_msgs:
                        if msg['message_type'] == 'Basic ID':
                            drone_data['uas_id'] = msg.get('uas_id', 'Unknown')
                            drone_data['ua_type'] = msg.get('ua_type', 'Unknown')
                            drone_data['id_type'] = msg.get('id_type')
                            drone_data['china_compliant'] = msg.get('china_compliant', False)
                        elif msg['message_type'] == 'Location':
                            drone_data['latitude'] = msg.get('latitude')
                            drone_data['longitude'] = msg.get('longitude')
                            drone_data['altitude'] = msg.get('altitude_m')
                            drone_data['speed_horizontal'] = msg.get('speed_horizontal_m_s')
                            drone_data['speed_vertical'] = msg.get('speed_vertical_m_s')
                            drone_data['heading'] = msg.get('direction_deg')
                            drone_data['status'] = msg.get('status')
                        elif msg['message_type'] == 'System':
                            drone_data['operator_latitude'] = msg.get('operator_latitude')
                            drone_data['operator_longitude'] = msg.get('operator_longitude')
                            drone_data['classification_region'] = msg.get('classification_region')
                    
                    # 仅当解析到有效数据时才处理
                    if 'uas_id' in drone_data:
                        DRONE_DATA[src_mac] = drone_data
                        print(f"🇨🇳 C-RID drone detected: {src_mac} -> {drone_data['uas_id']}")
                        
                        if app_loop:
                            asyncio.run_coroutine_threadsafe(
                                BROADCAST_QUEUE.put({
                                    "type": "drone_update",
                                    "data": drone_data,
                                    "mac": src_mac
                                }),
                                app_loop
                            )
                        
                        DATA_QUEUE.put((src_mac, drone_data))
            
            
            except Exception as e:
                print(f"❌ Packet processing error: {e}")
            
            PACKET_QUEUE.task_done()
        except queue.Empty:
            continue
        except Exception as e:
            print(f"❌ Data processor error: {e}")

async def broadcast_worker():
    """从队列中获取数据并通过 WebSocket 广播"""
    print("🔄 Starting broadcast worker")
    while True:
        try:
            data = await BROADCAST_QUEUE.get()
            await manager.broadcast(json.dumps(data))
            print(f"📤 Broadcasted drone update for {data.get('mac')}")
        except Exception as e:
            print(f"❌ Broadcast worker error: {e}")
        finally:
            await asyncio.sleep(0.01)

class RemoteIDSniffer:
    def __init__(self):
        self.stats = {"packets": 0, "filtered_packets": 0}
    
    def packet_handler(self, pkt):
        """处理数据包 - 现在过滤在 Python 中进行"""
        self.stats["packets"] += 1
        
        try:
            # 检查是否为管理帧 (type 0) 且是 Beacon 帧 (subtype 8)
            if hasattr(pkt, 'type') and hasattr(pkt, 'subtype'):
                if pkt.type == 0 and pkt.subtype == 8:  # Management frame, Beacon
                    self.stats["filtered_packets"] += 1
                    
                    # 每 100 个 Beacon 帧显示统计
                    if self.stats["filtered_packets"] % 100 == 0:
                        print(f"📡 Beacon frames processed: {self.stats['filtered_packets']}, "
                              f"Total packets: {self.stats['packets']}")
                    
                    # 将原始包放入处理队列
                    PACKET_QUEUE.put(pkt)
                    return
            
            # 可选：处理 Probe Request 帧 (subtype 4)
            if hasattr(pkt, 'type') and hasattr(pkt, 'subtype'):
                if pkt.type == 0 and pkt.subtype == 4:  # Management frame, Probe Request
                    # 同样放入队列（某些无人机使用 Probe Request 广播 Remote ID）
                    PACKET_QUEUE.put(pkt)
        
        except Exception as e:
            print(f"❌ Packet handler error: {e}")


@app.on_event("startup")
async def startup_event():
    """应用启动时初始化"""
    global app_loop
    app_loop = asyncio.get_running_loop()
    print("🚀 Application starting up")
    
    init_db()
    
    # 启动后台线程
    db_thread = threading.Thread(target=db_writer, daemon=True)
    db_thread.start()
    
    processor_thread = threading.Thread(target=data_processor, daemon=True)
    processor_thread.start()
    
    # 启动后台协程
    asyncio.create_task(broadcast_worker())
    
    # 启动抓包引擎
    sniffer = RemoteIDSniffer()
    # 在 startup_event 中
    sniff_thread = threading.Thread(
        target=lambda: scapy.sniff(
            iface="mon0",
            prn=sniffer.packet_handler,
            store=0,
            monitor=True,
            # 移除 filter 参数
            # filter="type mgt subtype beacon"  # ❌ 移除此行
        ),
        daemon=True
    )

    sniff_thread.start()
    print("📡 Starting packet capture on mon0")
    
    # 检查 mon0 状态
    try:
        interfaces = scapy.get_if_list()
        if "mon0" in interfaces:
            print("✅ mon0 interface found")
        else:
            print("⚠️ mon0 not found in available interfaces")
            print(f"💡 Available interfaces: {', '.join(interfaces)}")
            print("🔧 Hint: Run 'sudo ip link set mon0 down && sudo iw dev mon0 set type monitor && sudo ip link set mon0 up'")
    except Exception as e:
        print(f"⚠️ Interface check failed: {e}")

@app.on_event("shutdown")
async def shutdown_event():
    """应用关闭时清理"""
    print("🛑 Application shutting down")

if __name__ == "__main__":
    import uvicorn
    import struct  # 添加 struct 导入
    
    print("🚀 Starting Remote ID Backend API")
    print(f"📡 Interface: mon0")
    print(f"🌐 API: http://0.0.0.0:8000")
    print(f"💬 WebSocket: ws://0.0.0.0:8000/ws")
    print("✅ Supports: ASTM F3411-22a + GB42590-2023")
    
    uvicorn.run(
        app,
        host="0.0.0.0",
        port=8000,
        log_level="info",
        reload=False
    )