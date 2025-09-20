package api

import (
	"net/http"
	"time"

	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guttosm/b3pulse/internal/domain/dto"
	"github.com/guttosm/b3pulse/internal/service"
)

// Handler provides HTTP handlers for trade aggregation endpoints.
//
// Responsibilities:
//   - Validate incoming HTTP query parameters
//   - Interact with the repository layer for data access
//   - Translate repository results into response DTOs
//   - Return structured JSON responses with appropriate HTTP status codes
type Handler struct {
	svc service.AggregateService
}

// NewHandler constructs a new Handler instance.
//
// Parameters:
//   - repo (storage.TradesRepository): Repository dependency used for querying trade data.
//
// Returns:
//   - *Handler: A handler ready to be registered with the router.
func NewHandler(svc service.AggregateService) *Handler {
	return &Handler{svc: svc}
}

// GetAggregate handles GET /api/v1/aggregate requests.
//
// Query Parameters:
//   - ticker (string, required): Stock ticker symbol (e.g., "PETR4").
//   - data_inicio (string, optional): Minimum trade date in YYYY-MM-DD format.
//
// Responses:
//   - 200 OK: Returns AggregateResponse containing max price and max daily volume.
//   - 400 Bad Request: Missing or invalid query parameters.
//   - 404 Not Found: No trades found for the given ticker/date range.
//   - 500 Internal Server Error: Failure in repository or database layer.
//
// GetAggregate godoc
// @Summary      Get aggregate by ticker
// @Description  Returns max price and max daily volume for the given ticker since an optional start date
// @Tags         aggregate
// @Accept       json
// @Produce      json
// @Param        ticker       query     string  true   "Stock ticker" example(PETR4)
// @Param        data_inicio  query     string  false  "Start date in YYYY-MM-DD" example(2024-09-01)
// @Success      200          {object}  dto.AggregateResponse  "Success"
// @Failure      400          {object}  dto.ErrorResponse      "Bad Request"
// @Failure      404          {object}  dto.ErrorResponse      "Not Found"
// @Failure      500          {object}  dto.ErrorResponse      "Internal Error"
// @Router       /api/v1/aggregate [get]
func (h *Handler) GetAggregate(c *gin.Context) {
	// ─── Validate "ticker" param ──────────────────────────────
	ticker := strings.ToUpper(strings.TrimSpace(c.Query("ticker")))
	if ticker == "" {
		c.JSON(http.StatusBadRequest, dto.NewErrorResponse("ticker is required", nil))
		return
	}

	// ─── Parse optional "data_inicio" param ───────────────────
	var startDate *time.Time
	var endDate *time.Time
	if s := c.Query("data_inicio"); s != "" {
		parsed, err := time.Parse("2006-01-02", s)
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.NewErrorResponse("invalid data_inicio format, expected YYYY-MM-DD", err))
			return
		}
		startDate = &parsed
		// When provided, include data where trade_date >= data_inicio (no upper bound)
	} else {
		// Default: last 7 ingested days, ending yesterday
		today := time.Now().UTC()
		yday := today.AddDate(0, 0, -1)
		start := yday.AddDate(0, 0, -6)
		// normalize to date-only (strip time)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		yday = time.Date(yday.Year(), yday.Month(), yday.Day(), 0, 0, 0, 0, time.UTC)
		startDate = &start
		endDate = &yday
	}

	// ─── Query service (with request context) ─────────────────
	agg, err := h.svc.GetAggregate(c.Request.Context(), ticker, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.NewErrorResponse("failed to fetch aggregates", err))
		return
	}
	if agg == nil {
		c.JSON(http.StatusNotFound, dto.NewErrorResponse("no data found", nil))
		return
	}

	// ─── Build and return response DTO ────────────────────────
	resp := dto.AggregateResponse{
		Ticker:         agg.Ticker,
		MaxRangeValue:  agg.MaxRangeValue,
		MaxDailyVolume: agg.MaxDailyVolume,
	}

	c.JSON(http.StatusOK, resp)
}
