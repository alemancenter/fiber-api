package contentaudit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/alemancenter/fiber-api/internal/models"
	auditservice "github.com/alemancenter/fiber-api/internal/services/contentaudit"
	"github.com/alemancenter/fiber-api/internal/utils"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type Handler struct {
	svc *auditservice.Service
}

func New(svc *auditservice.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Start(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var userID *uint
	if user, ok := c.Locals("user").(*models.User); ok && user != nil {
		id := user.ID
		userID = &id
	}

	run, err := h.svc.Start(ctx, models.PolicyAuditTriggerManual, userID)
	if err != nil {
		if errors.Is(err, auditservice.ErrAlreadyRunning) {
			return utils.BadRequest(c, "content audit is already running")
		}
		return utils.InternalError(c, "failed to start content audit")
	}

	return utils.Created(c, "content audit started", run)
}

func (h *Handler) ListRuns(c *fiber.Ctx) error {
	pag := utils.GetPagination(c)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	runs, total, err := h.svc.ListRuns(ctx, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c, "failed to load content audit runs")
	}

	return utils.Paginated(c, "success", runs, pag.BuildMeta(total))
}

func (h *Handler) ShowRun(c *fiber.Ctx) error {
	runID, err := parseRunID(c)
	if err != nil {
		return utils.BadRequest(c, "invalid audit run id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	run, err := h.svc.GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "failed to load content audit run")
	}

	return utils.Success(c, "success", run)
}

func (h *Handler) ListFindings(c *fiber.Ctx) error {
	runID, err := parseRunID(c)
	if err != nil {
		return utils.BadRequest(c, "invalid audit run id")
	}

	pag := utils.GetPagination(c)
	risk := c.Query("risk")
	contentType := c.Query("type", c.Query("content_type"))
	search := c.Query("q")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	findings, total, err := h.svc.ListFindings(ctx, runID, risk, contentType, search, pag.PerPage, pag.Offset)
	if err != nil {
		return utils.InternalError(c, "failed to load content audit findings")
	}

	return utils.Paginated(c, "success", findings, pag.BuildMeta(total))
}

func (h *Handler) ExportCSV(c *fiber.Ctx) error {
	runID, err := parseRunID(c)
	if err != nil {
		return utils.BadRequest(c, "invalid audit run id")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var buf bytes.Buffer
	if err := h.svc.ExportCSV(ctx, runID, &buf); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.NotFound(c)
		}
		return utils.InternalError(c, "failed to export content audit report")
	}

	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="policy_audit_report_run_%d.csv"`, runID))
	return c.Send(buf.Bytes())
}

func parseRunID(c *fiber.Ctx) (uint64, error) {
	return strconv.ParseUint(c.Params("id"), 10, 64)
}
