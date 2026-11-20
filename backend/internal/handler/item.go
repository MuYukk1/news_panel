package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"news_panel/internal/store"
)

const maxListLimit = 100
const maxBatchUpdateIDs = 1000

type markItemsReadRequest struct {
	IDs []int64 `json:"ids" binding:"required"`
}

func (h *Handler) listItems(c *gin.Context) {
	params, valid := triageQueryParams(c)
	if !valid {
		return
	}

	if feedID := c.Query("feed_id"); feedID != "" {
		id, err := strconv.ParseInt(feedID, 10, 64)
		if err != nil {
			badRequestError(c, "invalid feed_id")
			return
		}
		params.FeedID = &id
	}

	if groupID := c.Query("group_id"); groupID != "" {
		id, err := strconv.ParseInt(groupID, 10, 64)
		if err != nil {
			badRequestError(c, "invalid group_id")
			return
		}
		params.GroupID = &id
	}

	if unread := c.Query("unread"); unread != "" {
		val, err := strconv.ParseBool(unread)
		if err != nil {
			badRequestError(c, "invalid unread")
			return
		}
		params.Unread = &val
	}

	if limit := c.Query("limit"); limit != "" {
		val, err := strconv.Atoi(limit)
		if err != nil || val <= 0 {
			badRequestError(c, "invalid limit")
			return
		}
		if val > maxListLimit {
			val = maxListLimit
		}
		params.Limit = val
	} else {
		params.Limit = 10
	}

	if before := c.Query("before"); before != "" {
		pubDate, id, err := parseCursor(before)
		if err != nil {
			badRequestError(c, "invalid before")
			return
		}
		params.BeforePubDate = &pubDate
		params.BeforeID = &id
	}

	if orderBy := c.Query("order_by"); orderBy != "" {
		params.OrderBy = orderBy
	} else {
		params.OrderBy = "pub_date"
	}

	// The cursor is keyed on pub_date, so it is only valid with the default ordering.
	if params.BeforePubDate != nil && params.OrderBy == "created_at" {
		badRequestError(c, "before cursor is only supported with default ordering (pub_date)")
		return
	}

	items, err := h.store.ListItems(params)
	if err != nil {
		internalError(c, err, "list items")
		return
	}

	total, err := h.store.CountItems(params)
	if err != nil {
		internalError(c, err, "count items")
		return
	}

	// A non-null next_cursor signals the client may request another full page.
	var nextCursor *string
	if params.Limit > 0 && len(items) >= params.Limit {
		last := items[len(items)-1]
		nc := fmt.Sprintf("%d_%d", last.PubDate, last.ID)
		nextCursor = &nc
	}
	paginatedListResponse(c, items, total, nextCursor)
}

func (h *Handler) getItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		badRequestError(c, "invalid id")
		return
	}

	item, err := h.store.GetItem(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			notFoundError(c, "item")
			return
		}
		internalError(c, err, "get item")
		return
	}

	dataResponse(c, item)
}

func (h *Handler) markItemsRead(c *gin.Context) {
	var req markItemsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequestError(c, "invalid request")
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > maxBatchUpdateIDs {
		badRequestError(c, "invalid ids")
		return
	}

	if err := h.store.BatchUpdateItemsUnread(req.IDs, false); err != nil {
		internalError(c, err, "mark items as read")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) markItemsUnread(c *gin.Context) {
	var req markItemsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequestError(c, "invalid request")
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > maxBatchUpdateIDs {
		badRequestError(c, "invalid ids")
		return
	}

	if err := h.store.BatchUpdateItemsUnread(req.IDs, true); err != nil {
		internalError(c, err, "mark items as unread")
		return
	}

	c.Status(http.StatusNoContent)
}

func triageQueryParams(c *gin.Context) (store.ListItemsParams, bool) {
	params := store.ListItemsParams{TriageStatus: c.Query("triage_status"), Domain: c.Query("domain"), Priority: c.Query("priority")}
	if params.TriageStatus != "" && !oneOf(params.TriageStatus, "pending", "kept", "filtered", "failed", "unscored") {
		badRequestError(c, "invalid triage_status")
		return params, false
	}
	if params.Priority != "" && !oneOf(params.Priority, "critical", "high", "normal", "low") {
		badRequestError(c, "invalid priority")
		return params, false
	}
	if score, exists := c.GetQuery("min_score"); exists {
		n, err := strconv.Atoi(score)
		if err != nil || n < 0 || n > 100 {
			badRequestError(c, "invalid min_score")
			return params, false
		}
		params.MinScore = &n
	}
	if scoredRaw, exists := c.GetQuery("scored"); exists {
		switch scoredRaw {
		case "true":
			v := true
			params.Scored = &v
		case "false":
			v := false
			params.Scored = &v
		default:
			badRequestError(c, "invalid scored")
			return params, false
		}
	}

	return params, true
}
