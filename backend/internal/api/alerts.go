package api

import (
	"net/http"
	"strconv"
	"time"

	"remoteid-monitor/pkg/types"

	"github.com/gin-gonic/gin"
)

func (s *Server) listAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	alerts, total := s.processor.GetAlerts(limit, offset) // 现在返回两个值
	if alerts == nil {
		alerts = []*types.Alert{}
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"count":  len(alerts),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) createAlert(c *gin.Context) {
	var alert types.Alert
	if err := c.ShouldBindJSON(&alert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_json",
			"message": "JSON 数据格式错误",
			"details": err.Error(),
		})
		return
	}

	alert.CreatedAt = time.Now()
	alert.Resolved = false

	createdAlert := s.processor.CreateAlert(&alert)
	c.JSON(http.StatusCreated, createdAlert)
}

func (s *Server) getAlertDetails(c *gin.Context) {
	id := c.Param("id")
	alert := s.processor.GetAlertByID(id)
	if alert == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "alert_not_found",
			"message": "指定的警报未找到",
			"id":      id,
		})
		return
	}
	c.JSON(http.StatusOK, alert)
}

func (s *Server) resolveAlert(c *gin.Context) {
	id := c.Param("id")
	result := s.processor.ResolveAlert(id)
	if !result {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "alert_not_found",
			"message": "指定的警报未找到或已解决",
			"id":      id,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "警报已解决",
		"id":      id,
	})
}

func (s *Server) getAlertStatistics(c *gin.Context) {
	stats := s.processor.GetAlertStatistics()
	c.JSON(http.StatusOK, stats)
}

func (s *Server) clearAlerts(c *gin.Context) {
	count := s.processor.ClearAllAlerts()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "所有警报已清除",
		"count":   count,
	})
}

func (s *Server) searchAlerts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter 'q' is required"})
		return
	}

	results := s.processor.SearchAlerts(query)
	if results == nil {
		results = []*types.Alert{}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}
