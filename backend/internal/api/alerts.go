// internal/api/alerts.go
package api

import (
	"net/http"
	"time"

	// 💡 引入原程序共享的 DTO 核心包
	// 💡 在前面加上 models 别名，这样代码里的 models.xxx 就全部合法了！
	"remoteid-monitor/pkg/types"

	"github.com/gin-gonic/gin"
)

func (s *Server) listAlerts(c *gin.Context) {
	limit := c.DefaultQuery("limit", "10")

	alerts := s.processor.GetAlerts(limit)
	if alerts == nil {
		alerts = []*types.Alert{}
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"count":  len(alerts),
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

	// 💡 修改后：将 result 改为 err，并检查是否为 nil
	err := s.processor.ResolveAlert(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "alert_not_found",
			"message": "指定的警报未找到",
			"id":      id,
		})
		return
	}

	c.JSON(http.StatusOK, err)
}

func (s *Server) resolveAlert(c *gin.Context) {
	id := c.Param("id")

	// 找到第 67-68 行，修改为：
	err := s.processor.ResolveAlert(id)
	if err != nil { // 👈 如果返回了错误，说明警报未找到或处理失败
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "alert_not_found",
			"message": "指定的警报未找到",
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

	results := s.processor.SearchAlerts(query)
	if results == nil {
		results = []*types.Alert{}
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}
