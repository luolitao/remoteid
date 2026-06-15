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
	c.JSON(http.StatusOK, gin.H{
		"drones": drones,
		"count":  len(drones),
	})
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
