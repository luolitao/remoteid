#!/usr/bin/env python3
from flask import Flask, request, jsonify, render_template_string, redirect, url_for
import serial, json
import glob
import serial.tools.list_ports as list_ports
import csv
from datetime import datetime
import atexit
ASCII_ART = r"""
       _____                .__              ________          __                 __     
      /     \   ____   _____|  |__           \______ \   _____/  |_  ____   _____/  |_   
     /  \ /  \_/ __ \ /  ___/  |  \   ______  |    |  \_/ __ \   __\/ __ \_/ ___\   __\  
    /    Y    \  ___/ \___ \|   Y  \ /_____/  |    `   \  ___/|  | \  ___/\  \___|  |    
    \____|__  /\___  >____  >___|  /         /_______  /\___  >__|  \___  >\___  >__|    
            \/     \/     \/     \/                  \/     \/          \/     \/        
________                                 _________                     _____             
\______ \_______  ____   ____   ____    /   _____/_____   ____   _____/ ____\___________ 
 |    |  \_  __ \/  _ \ /    \_/ __ \   \_____  \\____ \ /  _ \ /  _ \   __\/ __ \_  __ \
 |    `   \  | \(  <_> )   |  \  ___/   /        \  |_> >  <_> |  <_> )  | \  ___/|  | \/
/_______  /__|   \____/|___|  /\___  > /_______  /   __/ \____/ \____/|__|  \___  >__|   
        \/                  \/     \/          \/|__|                           \/       
"""

app = Flask(__name__)

# Serial setup
SERIAL_PORT = "/dev/ttyUSB0"
BAUD_RATE   = 115200
current_port = SERIAL_PORT
selected_port = None
try:
    ser = serial.Serial(current_port, BAUD_RATE, timeout=1)
except:
    ser = None

# Ensure serial port is closed when the script exits to stop any broadcasting
atexit.register(lambda: ser.close() if ser and ser.is_open else None)

# Log every mission path to CSV
PATH_CSV = "paths.csv"
try:
    with open(PATH_CSV, 'x', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['timestamp','path'])
except FileExistsError:
    pass

# In-memory mission storage
mission = {"basic_id": "", "drone_altitude": 0, "path": []}
# Track whether "play" has been pressed
play_active = False

# Port-selection template
PORT_SELECT_HTML = """
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Select USB Serial Port</title>
  <style>
    body { background-color: black; color: lime; font-family: monospace; text-align: center; margin: 0; padding: 0; height: 100vh; display: flex; align-items: center; justify-content: center; }
    .container { display: inline-block; text-align: left; }
    pre { font-size: 16px; margin-bottom: 20px; }
    form { display: flex; flex-direction: column; }
    select, button { background-color: #333; color: lime; border: none; padding: 8px; margin-top: 10px; font-size: 16px; cursor: pointer; }
    button { border: 1px solid lime; }
  </style>
</head>
<body>
  <div class="container">
    <pre>{{ ascii_art }}</pre>
    <h2 style="text-align:center; margin:0;">Select USB Serial Port</h2>
    <form method="post">
      <select name="serial_port">
        <option value="">--Select Port--</option>
        {% for p in ports %}
          <option value="{{ p }}">{{ p }}</option>
        {% endfor %}
      </select>
      <button type="submit">Continue</button>
    </form>
  </div>
</body>
</html>
"""

# HTML UI
HTML = """
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>Drone Spoofer</title>
<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"/>
<style>
  body, html { margin:0; padding:0; height:100%; background:black; color:lime; font-family:monospace; }
  #map { position:absolute; top:0; bottom:0; width:100%; }
  #control {
    position: absolute;
    top: 10px;
    right: 20px;
    background: rgba(0,0,0,0.8);
    padding: 15px;
    border: 1px solid lime;
    border-radius: 6px;
    z-index: 1000;
    width: 280px;
    box-sizing: border-box;
    text-align: center;
  }
  /* Removed #pilotControl styles */
  #serialStatus { position:absolute; bottom:10px; right:10px; background:rgba(0,0,0,0.8); padding:5px; border:1px solid lime; border-radius:6px; font-family:monospace; z-index:1000;}
  /* Uniform styling for all controls */
  #control label, #control input, #control select, #control button {
    display: block;
    width: 100%;
    margin: 8px 0;
    box-sizing: border-box;
  }
  #control label {
    text-align: left;
    margin-top: 0;
  }
  #control input, #control select {
    background: #222;
    color: lime;
    border: 1px solid lime;
    padding: 6px;
    font-family: monospace;
    font-size: 14px;
  }
  #control button {
    background: transparent;
    padding: 8px;
    font-size: 14px;
    cursor: pointer;
  }
  #control button#setPilot { background:transparent; border:1px solid #FF00FF !important; color:#FF00FF !important; cursor:pointer; }
  #control button#play { background:transparent; border:1px solid green !important; color:green !important; cursor:pointer; }
  #control button#pause { background:transparent; border:1px solid orange !important; color:orange !important; cursor:pointer; }
  #control button#stop { background:transparent; border:1px solid red !important; color:red !important; cursor:pointer; }
  .leaflet-control-attribution { display:none !important; }
  @keyframes flashPurple { 0%{background-color:purple;}100%{background-color:transparent;} }
  .flashPurple { animation:flashPurple 0.3s ease; background-color:purple !important; }
  #clearPathsContainer {
    position: absolute;
    bottom: 10px;
    left: 10px;
    z-index: 1000;
    text-align: center;
  }
  #clearPaths {
      background: purple;
      border: 1px solid lime;
      border-radius: 6px;
      color: lime;
      padding: 5px 10px;
      font-family: monospace;
      cursor: pointer;
  }
  #clearPaths:hover {
      background: purple;
      color: lime;
  }
</style>
<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
<script>
  function flash(id) {
    const btn = document.getElementById(id);
    if (!btn) return;
    btn.classList.add('flashPurple');
    setTimeout(()=>btn.classList.remove('flashPurple'),300);
  }
</script>
</head>
<body>
<div id="map"></div>
<div id="control">
  <div id="controlBody">
    <label style="color:#FF00FF;">Remote ID:
      <input id="basicId" placeholder="Remote ID"/>
    </label>
    <label style="color:#FF00FF;">Altitude (m):
      <input id="alt" type="number" value="100"/>
    </label>
    <label style="color:#FF00FF;">Speed (mph):
      <input id="speed" type="number" value="25"/>
    </label>
    <label style="color:#FF00FF; margin-top:8px;">Pilot ID:
      <input id="pilotId" placeholder="Pilot ID"/>
    </label>
    <button id="setPilot">Set Pilot Location</button>
    <small style="color:#7DF9FF; display:block; text-align:center; margin-top:4px;">
      Select pilot location before pressing play
    </small>
    <div style="display:flex; justify-content:space-between; margin-top:10px;">
      <button id="play" style="width:32%; margin:0;">Play</button>
      <button id="pause" style="width:32%; margin:0;">Pause</button>
      <button id="stop" style="width:32%; margin:0;">Stop</button>
    </div>
  </div>  <!-- end of controlBody -->
</div>  <!-- end of control -->
<div id="serialStatus"></div>
<div id="clearPathsContainer">
  <button id="clearPaths">Clear Paths</button>
</div>
<script>
  var map = L.map('map',{attributionControl:false}).setView([35.5961,-82.5552],16);
  L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',{maxZoom:19}).addTo(map);

  var waypointMarkers = [], path = [], poly = L.polyline(path,{color:'lime'}).addTo(map);
  var loopLine = null;
  var pilotSetMode=false,pilotMarker,droneMarker,simIndex=null;
  var statusCircle=null,serialInterval=null,playing=false;
  var firstHighlight=null,lastHighlight=null;
  var segmentOffset = 0;

  document.getElementById('setPilot').onclick=()=>{
    flash('setPilot');
    pilotSetMode=true;
    document.getElementById('setPilot').style.backgroundColor='#FF00FF';
  };

  map.on('click',e=>{
    if(pilotSetMode){
      let latlng=[e.latlng.lat,e.latlng.lng];
      if(pilotMarker)pilotMarker.setLatLng(latlng);
      else pilotMarker=L.marker(latlng,{icon:createIcon('👤')}).addTo(map);
      pilotSetMode=false;
      document.getElementById('setPilot').style.backgroundColor='';
    } else {
      path.push([e.latlng.lat,e.latlng.lng]);
      poly.setLatLngs(path);
      let wp=L.circleMarker([e.latlng.lat,e.latlng.lng],{radius:6,color:'#FF00FF',fillColor:'#FF00FF',fillOpacity:1}).addTo(map);
      waypointMarkers.push(wp);
      if(firstHighlight)map.removeLayer(firstHighlight);
      firstHighlight=L.circle(path[0],{radius:12,color:'lime',fill:false,weight:3}).addTo(map);
    }
  });

  function createIcon(e){return L.divIcon({html:'<div style="font-size:24px;">'+e+'</div>',className:'',iconSize:[30,30],iconAnchor:[15,15]});}

  document.getElementById('play').onclick = () => {
    flash('play');
    playing = true;
    // send start mission
    let missionData = {
      basic_id: document.getElementById('basicId').value,
      drone_altitude: parseInt(document.getElementById('alt').value) || 0,
      pilot_id: document.getElementById('pilotId').value,
      path: path
    };
    fetch('/api/start', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify(missionData)
    }).catch(console.error);

    if (!pilotMarker) return alert("Please set pilot location first.");

    // Initialize simIndex and segmentOffset if first play
    if (simIndex === null) {
      simIndex = 0;
      segmentOffset = 0;
      let startPoint = path[simIndex];
      if (droneMarker) droneMarker.setLatLng(startPoint);
      else droneMarker = L.marker(startPoint, {icon:createIcon('🛸')}).addTo(map);
    }
    // Status circle color = green for playing
    if (statusCircle) map.removeLayer(statusCircle);
    statusCircle = L.circleMarker(droneMarker.getLatLng(), {radius:16, color:'green', fill:false, weight:3}).addTo(map);
    // Highlight last waypoint blue
    if (lastHighlight) map.removeLayer(lastHighlight);
    lastHighlight = L.circle(path[path.length-1], {radius:12, color:'blue', fill:false, weight:3}).addTo(map);
    // draw loop-from-last back-to-start line as a solid light blue segment
    if (path.length > 1) {
      if (loopLine) map.removeLayer(loopLine);
      loopLine = L.polyline([ path[path.length-1], path[0] ], {
        color: 'lightblue'
      }).addTo(map);
    }

    // compute speed m/s
    let mph = parseFloat(document.getElementById('speed').value) || 25;
    let speedMps = mph * 0.44704;

    // begin or resume animation
    moveSegment(simIndex, segmentOffset, speedMps);

    // start serial updates
    if (serialInterval) clearInterval(serialInterval);
    serialInterval = setInterval(() => {
      if (droneMarker) {
        let d = droneMarker.getLatLng(), p = pilotMarker.getLatLng();
        let payload = {
          basic_id: document.getElementById('basicId').value,
          pilot_id: document.getElementById('pilotId').value,
          drone_altitude: parseInt(document.getElementById('alt').value) || 0,
          drone_lat: d.lat, drone_long: d.lng,
          pilot_lat: p.lat, pilot_long: p.lng
        };
        fetch('/api/update', {
          method: 'POST',
          headers: {'Content-Type':'application/json'},
          body: JSON.stringify(payload)
        }).catch(console.error);
      }
    }, 200);
  };

  function moveSegment(idx, offset, speedMps) {
    if (!playing) return;
    simIndex = idx;
    let from = path[idx], to = path[(idx+1)%path.length];
    let dist = map.distance(L.latLng(from), L.latLng(to));
    let duration = (dist / speedMps) * 1000;
    let startTime = performance.now();
    function step() {
      if (!playing) return;
      let elapsed = performance.now() - startTime;
      let t = offset + elapsed / duration;
      if (t >= 1) {
        // move to next segment
        segmentOffset = 0;
        let nextIdx = (idx + 1) % path.length;
        moveSegment(nextIdx, 0, speedMps);
      } else {
        // interpolate position
        let lat = from[0] + (to[0] - from[0]) * t;
        let lng = from[1] + (to[1] - from[1]) * t;
        droneMarker.setLatLng([lat, lng]);
        statusCircle.setLatLng([lat, lng]);
        segmentOffset = t;
        requestAnimationFrame(step);
      }
    }
    requestAnimationFrame(step);
  }

  document.getElementById('pause').onclick = () => {
    flash('pause');
    // Stop movement animation, but keep serialInterval and play_active true
    playing = false;
    // Do not call pause API or clear serialInterval
    if (statusCircle) map.removeLayer(statusCircle);
    statusCircle = L.circleMarker(droneMarker.getLatLng(), {radius:16, color:'orange', fill:false, weight:3}).addTo(map);
  };

  document.getElementById('stop').onclick=()=>{
    flash('stop');
    playing=false;
    fetch('/api/stop',{method:'POST'}).catch(console.error);
    if(serialInterval){clearInterval(serialInterval);serialInterval=null;}
    simIndex=null;
    if(statusCircle)map.removeLayer(statusCircle);
    statusCircle=L.circleMarker(droneMarker.getLatLng(),{radius:16,color:'red',fill:false,weight:3}).addTo(map);
  };

  function updateSerialStatus(){
    fetch('/api/serial_status').then(r=>r.json()).then(s=>{
      const statusEl = document.getElementById('serialStatus');
      const portLabel = 'Port:' + (s.port||'none') + ' – ';
      const statusLabel = s.connected
        ? '<span style="color:lime">Connected</span>'
        : '<span style="color:red">Disconnected</span>';
      statusEl.innerHTML = portLabel + statusLabel;
    });
  }
  setInterval(updateSerialStatus,1000);
  updateSerialStatus();
  // Clear all waypoints and path data
  document.getElementById('clearPaths').onclick = () => {
    // Remove existing waypoint markers
    waypointMarkers.forEach(m => map.removeLayer(m));
    waypointMarkers = [];
    // Clear the polyline
    path = [];
    poly.setLatLngs(path);
    // Remove loop line, first and last highlights
    if (loopLine) { map.removeLayer(loopLine); loopLine = null; }
    if (firstHighlight) { map.removeLayer(firstHighlight); firstHighlight = null; }
    if (lastHighlight) { map.removeLayer(lastHighlight); lastHighlight = null; }
    // Remove drone marker and status circle
    if (droneMarker) { map.removeLayer(droneMarker); droneMarker = null; }
    if (statusCircle) { map.removeLayer(statusCircle); statusCircle = null; }
    // Reset simulation state
    simIndex = null;
    segmentOffset = 0;
    playing = false;
  };
</script>
</body>
</html>
"""

@app.route('/', methods=['GET','POST'])
def index():
    global selected_port, ser, current_port
    ports = [p.device for p in list_ports.comports()]
    if request.method == 'POST':
        selected_port = request.form['serial_port']
        current_port = selected_port
        try:
            if ser and ser.is_open: ser.close()
        except: pass
        try:
            ser = serial.Serial(current_port, BAUD_RATE, timeout=1)
        except:
            ser = None
        return redirect(url_for('map_view'))
    return render_template_string(PORT_SELECT_HTML, ports=ports, ascii_art=ASCII_ART)

@app.route('/map')
def map_view():
    global selected_port
    if not selected_port:
        return redirect(url_for('index'))
    return render_template_string(HTML)

@app.route('/api/start', methods=['POST'])
def start():
    global ser, play_active
    play_active = True
    data = request.get_json()
    if play_active and ser:
        try:
            ser.write((json.dumps(data)+"\n").encode('ascii'))
        except Exception:
            ser = None

    # Log mission path to CSV
    try:
        with open(PATH_CSV, 'a', newline='') as f:
            writer = csv.writer(f)
            writer.writerow([datetime.now().isoformat(), json.dumps(data.get('path', []))])
    except Exception as e:
        print("Error writing path to CSV:", e)
    return jsonify(status='ok')

@app.route('/api/pause', methods=['POST'])
def pause_api():
    global play_active, ser
    was_playing = play_active
    # play_active = False
    if was_playing and ser:
        try:
            ser.write((json.dumps({"action":"pause"})+"\n").encode('ascii'))
        except Exception:
            ser = None
    return jsonify(status='paused')

@app.route('/api/stop', methods=['POST'])
def stop():
    global play_active, ser
    was_playing = play_active
    play_active = False
    if was_playing and ser:
        try:
            ser.write((json.dumps({"action":"stop"})+"\n").encode('ascii'))
            ser.write((json.dumps({"path":[]})+"\n").encode('ascii'))
        except Exception:
            ser = None
    return jsonify(status='stopped')

@app.route('/api/update', methods=['POST'])
def update_position():
    global ser, play_active
    if not play_active:
        return jsonify(status='ignored, not playing'), 200
    data = request.get_json()
    if ser:
        try:
            ser.write((json.dumps(data)+"\n").encode('ascii'))
        except Exception:
            ser = None
    return jsonify(status='ok')

@app.route('/api/serial_status', methods=['GET'])
def serial_status():
    global ser, selected_port
    # Get current plugged-in ports
    available = [p.device for p in list_ports.comports()]
    # If selected port not present, close any open serial and mark disconnected
    if selected_port not in available:
        try:
            if ser and ser.is_open:
                ser.close()
        except Exception:
            pass
        ser = None
        connected = False
    else:
        # Port present: ensure serial is open
        if ser is None or not ser.is_open:
            try:
                ser = serial.Serial(selected_port, BAUD_RATE, timeout=1)
            except Exception:
                ser = None
        connected = bool(ser and ser.is_open)
    return jsonify({"port": selected_port, "connected": connected})

if __name__=='__main__':
    app.run(host='0.0.0.0', port=5000)
