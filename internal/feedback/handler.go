package feedback

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bhyland-usda/job-portal/internal/middleware"
	"github.com/bhyland-usda/job-portal/internal/notification"
)

type FeedbackRequest struct {
	ID            string
	RequesterID   string
	RequesterName string
	ReviewerID    string
	ReviewerName  string
	Subject       string
	Feedback      string
	Status        string
	CreatedAt     time.Time
}

type FeedbackPage struct {
	middleware.BaseData
	Tab      string
	Sent     []FeedbackRequest
	Received []FeedbackRequest
}

type RequestPage struct {
	middleware.BaseData
	ReceiverID   string
	ReceiverName string
	Error        string
}

type RespondPage struct {
	middleware.BaseData
	Request FeedbackRequest
	Error   string
}

type Handler struct {
	db    *sql.DB
	pages map[string]*template.Template
	notif *notification.Handler
}

func NewHandler(db *sql.DB, pages map[string]*template.Template, notif *notification.Handler) *Handler {
	return &Handler{db: db, pages: pages, notif: notif}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /feedback", requireAuth(http.HandlerFunc(h.showFeedback)))
	mux.Handle("GET /feedback-ask/{userID}", requireAuth(http.HandlerFunc(h.showRequest)))
	mux.Handle("POST /feedback-ask/{userID}", requireAuth(http.HandlerFunc(h.handleRequest)))
	mux.Handle("GET /feedback-respond/{id}", requireAuth(http.HandlerFunc(h.showRespond)))
	mux.Handle("POST /feedback-respond/{id}", requireAuth(http.HandlerFunc(h.handleRespond)))
	mux.Handle("POST /feedback-decline/{id}", requireAuth(http.HandlerFunc(h.handleDecline)))
}

func (h *Handler) showFeedback(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "received"
	}

	// Load sent requests
	sentRows, err := h.db.QueryContext(r.Context(),
		`SELECT fr.id, fr.requester_id,
			CONCAT(rq.first_name, ' ', rq.last_name),
			fr.reviewer_id,
			CONCAT(rv.first_name, ' ', rv.last_name),
			fr.subject, COALESCE(fr.feedback, ''), fr.status, fr.created_at
		 FROM feedback_requests fr
		 JOIN users rq ON rq.id = fr.requester_id
		 JOIN users rv ON rv.id = fr.reviewer_id
		 WHERE fr.requester_id = $1
		 ORDER BY fr.created_at DESC`, userID,
	)
	if err != nil {
		slog.Error("failed to load sent feedback", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer sentRows.Close()

	var sent []FeedbackRequest
	for sentRows.Next() {
		var f FeedbackRequest
		if err := sentRows.Scan(&f.ID, &f.RequesterID, &f.RequesterName,
			&f.ReviewerID, &f.ReviewerName, &f.Subject, &f.Feedback,
			&f.Status, &f.CreatedAt); err != nil {
			slog.Error("failed to scan sent feedback", "error", err)
			continue
		}
		sent = append(sent, f)
	}

	// Load received requests
	recvRows, err := h.db.QueryContext(r.Context(),
		`SELECT fr.id, fr.requester_id,
			CONCAT(rq.first_name, ' ', rq.last_name),
			fr.reviewer_id,
			CONCAT(rv.first_name, ' ', rv.last_name),
			fr.subject, COALESCE(fr.feedback, ''), fr.status, fr.created_at
		 FROM feedback_requests fr
		 JOIN users rq ON rq.id = fr.requester_id
		 JOIN users rv ON rv.id = fr.reviewer_id
		 WHERE fr.reviewer_id = $1
		 ORDER BY fr.created_at DESC`, userID,
	)
	if err != nil {
		slog.Error("failed to load received feedback", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer recvRows.Close()

	var received []FeedbackRequest
	for recvRows.Next() {
		var f FeedbackRequest
		if err := recvRows.Scan(&f.ID, &f.RequesterID, &f.RequesterName,
			&f.ReviewerID, &f.ReviewerName, &f.Subject, &f.Feedback,
			&f.Status, &f.CreatedAt); err != nil {
			slog.Error("failed to scan received feedback", "error", err)
			continue
		}
		received = append(received, f)
	}

	data := FeedbackPage{
		BaseData: middleware.NewBaseData(r),
		Tab:      tab,
		Sent:     sent,
		Received: received,
	}

	if err := h.pages["feedback.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render feedback", "error", err)
	}
}

func (h *Handler) showRequest(w http.ResponseWriter, r *http.Request) {
	receiverID := r.PathValue("userID")

	var firstName, lastName string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, receiverID,
	).Scan(&firstName, &lastName)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data := RequestPage{
		BaseData:     middleware.NewBaseData(r),
		ReceiverID:   receiverID,
		ReceiverName: firstName + " " + lastName,
	}

	if err := h.pages["feedback_request.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render feedback request form", "error", err)
	}
}

func (h *Handler) handleRequest(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	receiverID := r.PathValue("userID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	subject := strings.TrimSpace(r.FormValue("subject"))
	if subject == "" {
		var firstName, lastName string
		_ = h.db.QueryRowContext(r.Context(),
			`SELECT first_name, last_name FROM users WHERE id = $1`, receiverID,
		).Scan(&firstName, &lastName)

		data := RequestPage{
			BaseData:     middleware.NewBaseData(r),
			ReceiverID:   receiverID,
			ReceiverName: firstName + " " + lastName,
			Error:        "Subject is required",
		}
		h.pages["feedback_request.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO feedback_requests (requester_id, reviewer_id, subject)
		 VALUES ($1, $2, $3)`,
		userID, receiverID, subject,
	)
	if err != nil {
		slog.Error("failed to create feedback request", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Notify reviewer
	var senderFirst, senderLast string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&senderFirst, &senderLast)

	notifMsg := senderFirst + " " + senderLast + " requested your feedback: " + subject
	if len(notifMsg) > 200 {
		notifMsg = notifMsg[:197] + "..."
	}
	h.notif.CreateNotification(r.Context(), receiverID, userID, "feedback", notifMsg)

	http.Redirect(w, r, "/feedback?tab=sent", http.StatusSeeOther)
}

func (h *Handler) showRespond(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	requestID := r.PathValue("id")

	var f FeedbackRequest
	err := h.db.QueryRowContext(r.Context(),
		`SELECT fr.id, fr.requester_id,
			CONCAT(rq.first_name, ' ', rq.last_name),
			fr.reviewer_id,
			CONCAT(rv.first_name, ' ', rv.last_name),
			fr.subject, COALESCE(fr.feedback, ''), fr.status, fr.created_at
		 FROM feedback_requests fr
		 JOIN users rq ON rq.id = fr.requester_id
		 JOIN users rv ON rv.id = fr.reviewer_id
		 WHERE fr.id = $1`,
		requestID,
	).Scan(&f.ID, &f.RequesterID, &f.RequesterName,
		&f.ReviewerID, &f.ReviewerName, &f.Subject,
		&f.Feedback, &f.Status, &f.CreatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Only the reviewer can respond
	if f.ReviewerID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if f.Status != "pending" {
		http.Redirect(w, r, "/feedback?tab=received", http.StatusSeeOther)
		return
	}

	data := RespondPage{BaseData: middleware.NewBaseData(r), Request: f}
	if err := h.pages["feedback_respond.html"].ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("failed to render feedback respond form", "error", err)
	}
}

func (h *Handler) handleRespond(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	requestID := r.PathValue("id")

	// Verify reviewer
	var reviewerID, requesterID, subject string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT reviewer_id, requester_id, subject FROM feedback_requests
		 WHERE id = $1 AND status = 'pending'`,
		requestID,
	).Scan(&reviewerID, &requesterID, &subject)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if reviewerID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	feedbackText := strings.TrimSpace(r.FormValue("feedback"))
	if feedbackText == "" {
		// Reload request for re-render
		var f FeedbackRequest
		h.db.QueryRowContext(r.Context(),
			`SELECT fr.id, fr.requester_id,
				CONCAT(rq.first_name, ' ', rq.last_name),
				fr.reviewer_id,
				CONCAT(rv.first_name, ' ', rv.last_name),
				fr.subject, COALESCE(fr.feedback, ''), fr.status, fr.created_at
			 FROM feedback_requests fr
			 JOIN users rq ON rq.id = fr.requester_id
			 JOIN users rv ON rv.id = fr.reviewer_id
			 WHERE fr.id = $1`,
			requestID,
		).Scan(&f.ID, &f.RequesterID, &f.RequesterName,
			&f.ReviewerID, &f.ReviewerName, &f.Subject,
			&f.Feedback, &f.Status, &f.CreatedAt)

		data := RespondPage{
			BaseData: middleware.NewBaseData(r),
			Request:  f,
			Error:    "Feedback text is required",
		}
		h.pages["feedback_respond.html"].ExecuteTemplate(w, "base", data)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE feedback_requests SET feedback = $1, status = 'completed', updated_at = NOW()
		 WHERE id = $2`,
		feedbackText, requestID,
	)
	if err != nil {
		slog.Error("failed to submit feedback", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Notify requester
	var reviewerFirst, reviewerLast string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&reviewerFirst, &reviewerLast)

	notifMsg := reviewerFirst + " " + reviewerLast + " completed your feedback request: " + subject
	if len(notifMsg) > 200 {
		notifMsg = notifMsg[:197] + "..."
	}
	h.notif.CreateNotification(r.Context(), requesterID, userID, "feedback", notifMsg)

	http.Redirect(w, r, "/feedback?tab=received", http.StatusSeeOther)
}

func (h *Handler) handleDecline(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	requestID := r.PathValue("id")

	var reviewerID, requesterID, subject string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT reviewer_id, requester_id, subject FROM feedback_requests
		 WHERE id = $1 AND status = 'pending'`,
		requestID,
	).Scan(&reviewerID, &requesterID, &subject)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if reviewerID != userID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE feedback_requests SET status = 'declined', updated_at = NOW()
		 WHERE id = $1`,
		requestID,
	)
	if err != nil {
		slog.Error("failed to decline feedback", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Notify requester
	var reviewerFirst, reviewerLast string
	_ = h.db.QueryRowContext(r.Context(),
		`SELECT first_name, last_name FROM users WHERE id = $1`, userID,
	).Scan(&reviewerFirst, &reviewerLast)

	notifMsg := reviewerFirst + " " + reviewerLast + " declined your feedback request: " + subject
	if len(notifMsg) > 200 {
		notifMsg = notifMsg[:197] + "..."
	}
	h.notif.CreateNotification(r.Context(), requesterID, userID, "feedback", notifMsg)

	http.Redirect(w, r, "/feedback?tab=received", http.StatusSeeOther)
}
