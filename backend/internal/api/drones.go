package api

import (
	"net/http"
	"remoteid-monitor/pkg/types"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) listDrones(c *gin.Context) {
	drones := s.processor.GetAllDrones()
	if drones == nil {
		drones = []*types.DroneData{}
	}

	// 富化数据，提升 GB46750 特有字段到顶层
	enriched := make([]gin.H, len(drones))
	for i, d := range drones {
		enriched[i] = enrichDroneForAPI(d)
	}

	c.JSON(http.StatusOK, gin.H{
		"drones": enriched,
		"count":  len(enriched),
	})
}

// enrichDroneForAPI 将协议特定数据提升到顶层，方便前端显示
func enrichDroneForAPI(d *types.DroneData) gin.H {
	result := gin.H{
		"mac":                   d.MAC,
		"uas_id":                d.UASID,
		"operator_id":           d.OperatorID,
		"ua_type":               d.UAType,
		"id_type":               d.IDType,
		"latitude":              d.Latitude,
		"longitude":             d.Longitude,
		"altitude":              d.Altitude,
		"speed":                 d.Speed,
		"heading":               d.Heading,
		"speed_v":               d.SpeedVertical,
		"flight_status":         d.FlightStatus,
		"h_accuracy":            d.HAccuracy,
		"v_accuracy":            d.VAccuracy,
		"s_accuracy":            d.SAccuracy,
		"timestamp":             d.LocationTimestamp,
		"operator_latitude":     d.OperatorLatitude,
		"operator_longitude":    d.OperatorLongitude,
		"operator_altitude":     d.OperatorAltitude,
		"classification_region": d.Classification,
		"area_radius_m":         d.AreaRadiusM,
		"standard":              d.Standard,
		"source":                d.Source,
		"signal_strength":       d.SignalStrength,
		"battery_level":         d.BatteryLevel,
		"flight_time":           d.FlightTime,
		"first_seen":            d.FirstSeen,
		"last_seen":             d.LastSeen,
		"protocol":              d.Protocol,
	}

	// 如果是 GB46750 协议，回填缺失的通用字段
	if d.Protocol == "gb46750" && d.GB46750 != nil {
		gb := d.GB46750

		// 回填 ua_type（若顶层为空）
		if result["ua_type"] == "" && gb.UACategory != "" {
			result["ua_type"] = gb.UACategory
		}
		// 回填 operator_id（若顶层为空）
		if result["operator_id"] == "" && gb.RealNameID != "" {
			result["operator_id"] = gb.RealNameID
		}
		// 回填 operator_latitude / longitude（若顶层为 0）
		if d.OperatorLatitude == 0 && gb.RCSLocation != nil && gb.RCSLocation.Latitude != 0 {
			result["operator_latitude"] = gb.RCSLocation.Latitude
			result["operator_longitude"] = gb.RCSLocation.Longitude
		}
		// 回填 operator_altitude（若顶层为 0）
		if d.OperatorAltitude == 0 && gb.RCSAltitude != 0 {
			result["operator_altitude"] = gb.RCSAltitude
		}
		// 回填 flight_status（若顶层为空）
		if result["flight_status"] == "" && gb.Status != "" {
			result["flight_status"] = gb.Status
		}
		// 可继续补充其他字段...
	}

	return result
}

func (s *Server) getDroneDetails(c *gin.Context) {
	mac := c.Param("mac")
	drone := s.processor.GetDroneByMAC(mac)
	if drone == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "drone_not_found",
			"message": "指定的无人机未找到",
			"mac":     mac,
		})
		return
	}
	c.JSON(http.StatusOK, drone)
}

func (s *Server) getTrajectory(c *gin.Context) {
	mac := c.Param("mac")

	// 获取 hours 参数，默认查询最近 24 小时，最大支持 720 小时 (30天)
	hoursStr := c.DefaultQuery("hours", "24")
	hours := 24
	if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 720 {
		hours = h
	}

	trajectory := s.processor.GetDroneTrajectory(mac, hours)

	// 如果轨迹为空或不存在，返回 404
	if trajectory == nil || len(trajectory.Points) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "trajectory_not_found",
			"message": "指定无人机的轨迹未找到或无历史数据",
			"mac":     mac,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trajectory": trajectory,
		"points":     len(trajectory.Points),
	})
}

func (s *Server) exportDroneData(c *gin.Context) {
	mac := c.Param("mac")
	exportData := s.processor.ExportDroneData(mac)
	if exportData == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "data_not_found",
			"message": "指定无人机的数据未找到",
			"mac":     mac,
		})
		return
	}
	c.JSON(http.StatusOK, exportData)
}

func (s *Server) searchDrones(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	results := s.processor.SearchDrones(query)
	if results == nil {
		results = []*types.DroneData{}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

func (s *Server) getDroneStatistics(c *gin.Context) {
	stats := s.processor.GetDroneStatistics()
	c.JSON(http.StatusOK, stats)
}
