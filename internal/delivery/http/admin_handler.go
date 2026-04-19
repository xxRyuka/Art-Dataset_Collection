package http

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryuka/art-dataset-collector/internal/usecase"
)

type AdminHandler struct {
	ratingUC *usecase.RatingUseCase
}

func NewAdminHandler(ratingUC *usecase.RatingUseCase) *AdminHandler {
	return &AdminHandler{ratingUC: ratingUC}
}

// GetChartData returns the full dashboard stats in JSON format.
func (h *AdminHandler) GetChartData(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.ratingUC.GetDashboardStats(ctx)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ExportCSV streams the database ratings directly as a CSV file to the client.
func (h *AdminHandler) ExportCSV(c *gin.Context) {
	ctx := c.Request.Context()

	exports, err := h.ratingUC.GetAllExports(ctx)
	if err != nil {
		respondWithError(c, err)
		return
	}

	// Set headers for file download
	timestamp := time.Now().Format("20060102_150405")
	fileName := "dataset_export_" + timestamp + ".csv"
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "text/csv")
	c.Header("Transfer-Encoding", "chunked")

	// Create CSV writer directly on the response writer
	writer := csv.NewWriter(c.Writer)

	// Write Headers
	headers := []string{
		"session_id",
		"file_name",
		"drive_file_id",
		"score",
		"age",
		"gender",
		"city",
		"knows_artist",
		"follows_artist",
		"rated_at",
	}
	if err := writer.Write(headers); err != nil {
		return
	}

	// Write Rows
	for _, e := range exports {
		knowsArtistStr := "false"
		if e.KnowsArtist {
			knowsArtistStr = "true"
		}

		followsArtistStr := "false"
		if e.FollowsArtist {
			followsArtistStr = "true"
		}

		record := []string{
			e.SessionID,
			e.FileName,
			e.DriveFileID,
			strconv.Itoa(e.Score),
			strconv.Itoa(e.Age),
			e.Gender,
			e.City,
			knowsArtistStr,
			followsArtistStr,
			e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}

		if err := writer.Write(record); err != nil {
			return // Cannot recover if connection drops
		}
	}

	writer.Flush()
}
