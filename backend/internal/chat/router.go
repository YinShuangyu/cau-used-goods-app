package chat

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, handler *Handler, authMiddleware, readableMiddleware, verifiedMiddleware gin.HandlerFunc) {
	group := r.Group("/chat")
	group.Use(authMiddleware, readableMiddleware)
	{
		group.POST("/conversations", verifiedMiddleware, handler.CreateOrGetConversation)
		group.GET("/conversations", handler.ListConversations)
		group.GET("/conversations/:id/messages", handler.ListMessages)
		group.POST("/conversations/:id/messages", verifiedMiddleware, handler.SendMessage)
		group.PUT("/conversations/:id/read", handler.MarkRead)
		group.GET("/conversations/:id/product", handler.GetConversationProduct)
	}
}
