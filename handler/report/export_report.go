package report

import (
	"fmt"
	"strconv"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/usecase"
	"github.com/gofiber/fiber/v3"
)

func (h *Handler) ExportReport(c fiber.Ctx) error {
	var (
		shopIDStr    = c.FormValue("shop_id")
		startTimeStr = c.FormValue("start_time")
		endTimeStr   = c.FormValue("end_time")
	)

	shopID, err := strconv.ParseInt(shopIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid shop_id",
		})
	}

	startTime, err := time.Parse(time.RFC3339Nano, startTimeStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid start time",
		})
	}

	endTime, err := time.Parse(time.RFC3339Nano, endTimeStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid end time",
		})
	}

	reportFile, err := h.reportUseCase.ExportReport(c.Context(), usecase.ExportReportParams{
		ShopID:    shopID,
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err,
		})
	}

	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"%s\"", reportFile.FileName))
	c.Set(fiber.HeaderContentType, fiber.MIMEOctetStream)
	return c.SendStream(reportFile.File)
}
