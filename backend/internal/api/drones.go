// internal/api/drones.go
package api

import (
	"net/http"
	"remoteid-monitor/pkg/types"

	"github.com/gin-gonic/gin"
)

func (s *Server) listDrones(c *gin.Context) {
	// 获取所有活动无人机，确保返回空数组而非 null
	drones := s.processor.GetAllDrones()
	if drones == nil {
		drones = []*types.DroneData{}
	}

	c.JSON(http.StatusOK, gin.H{
		"drones": drones,
		"count":  len(drones),
		"total":  len(drones),
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

	trajectory := s.processor.GetDroneTrajectory(mac)
	if trajectory == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "trajectory_not_found",
			"message": "指定无人机的轨迹未找到",
			"mac":     mac,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trajectory": trajectory,
		"points":     len(trajectory),
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

	results := s.processor.SearchDrones(query)
	if results == nil {
		results = []types.DroneDetail{}
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
