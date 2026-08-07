package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

func massageEnrollmentKey(cycleID, userID string) string { return cycleID + "\x00" + userID }
func (m *Memory) ListMassageCycles(context.Context) ([]domain.MassageCycle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.MassageCycle{}
	for _, v := range m.massageCycles {
		v.EligibleCount = len(m.massageEligible[v.ID])
		for _, e := range m.massageEnrollments {
			if e.CycleID == v.ID && e.Status == "enrolled" {
				v.EnrolledCount++
			}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServiceMonth > out[j].ServiceMonth })
	return out, nil
}
func (m *Memory) GetMassageCycle(_ context.Context, id string) (domain.MassageCycle, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.massageCycles[id]
	if !ok {
		return v, ErrNotFound
	}
	v.EligibleCount = len(m.massageEligible[id])
	for _, e := range m.massageEnrollments {
		if e.CycleID == id && e.Status == "enrolled" {
			v.EnrolledCount++
		}
	}
	return v, nil
}
func (m *Memory) CreateMassageCycle(_ context.Context, v domain.MassageCycle) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.massageCycles[v.ID]; ok {
		return ErrConflict
	}
	m.massageCycles[v.ID] = v
	return nil
}
func (m *Memory) UpdateMassageCycle(_ context.Context, v domain.MassageCycle, eligibleUserIDs []string, deliveries []domain.MassageDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.massageCycles[v.ID]
	if !ok {
		return ErrNotFound
	}
	if old.Status == "completed" || old.Status == "cancelled" {
		return ErrConflict
	}
	requestedSessions := make(map[string]domain.MassageSession, len(v.Sessions))
	for _, session := range v.Sessions {
		requestedSessions[session.ID] = session
	}
	mergedSessions := make([]domain.MassageSession, 0, len(old.Sessions))
	for _, oldSession := range old.Sessions {
		updated, exists := requestedSessions[oldSession.ID]
		if !exists {
			return ErrConflict
		}
		occupied := oldSession.CompletedCount
		for _, call := range m.massageCalls {
			if call.SessionID == oldSession.ID && (call.Status == "pending_delivery" || call.Status == "awaiting_response" || call.Status == "accepted") {
				occupied++
			}
		}
		if updated.Quota < occupied {
			return ErrConflict
		}
		updated.Status = oldSession.Status
		updated.CompletedCount = oldSession.CompletedCount
		updated.CreatedAt = oldSession.CreatedAt
		mergedSessions = append(mergedSessions, updated)
	}
	v.Sessions = mergedSessions
	v.Status = old.Status
	v.NextQueue = old.NextQueue
	v.CreatedAt = old.CreatedAt
	m.massageCycles[v.ID] = v
	if old.Status != domain.MassageCycleDraft {
		wanted := make(map[string]bool, len(eligibleUserIDs))
		for _, userID := range eligibleUserIDs {
			wanted[userID] = true
		}
		if m.massageEligible[v.ID] == nil {
			m.massageEligible[v.ID] = map[string]bool{}
		}
		for userID := range m.massageEligible[v.ID] {
			_, enrolled := m.massageEnrollments[massageEnrollmentKey(v.ID, userID)]
			if !wanted[userID] && !enrolled {
				delete(m.massageEligible[v.ID], userID)
			}
		}
		deliveryByUser := make(map[string]domain.MassageDelivery, len(deliveries))
		for _, delivery := range deliveries {
			deliveryByUser[delivery.UserID] = delivery
		}
		for userID := range wanted {
			if m.massageEligible[v.ID][userID] {
				continue
			}
			m.massageEligible[v.ID][userID] = true
			if delivery, exists := deliveryByUser[userID]; exists {
				m.massageDeliveries[delivery.ID] = delivery
			}
		}
		for id, delivery := range m.massageDeliveries {
			if delivery.CycleID == v.ID && (delivery.Kind == "signup" || delivery.Kind == "signup_resend") && delivery.Status == "pending" {
				delivery.AvailableAt = v.SignupNoticeAt
				delivery.UpdatedAt = v.UpdatedAt
				m.massageDeliveries[id] = delivery
			}
		}
	}
	return nil
}

func (m *Memory) DeleteMassageCycle(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cycle, exists := m.massageCycles[id]
	if !exists {
		return ErrNotFound
	}
	delete(m.massageCycles, id)
	delete(m.massageEligible, id)
	sessionIDs := map[string]bool{}
	enrollmentIDs := map[string]bool{}
	for _, session := range cycle.Sessions {
		sessionIDs[session.ID] = true
	}
	for key, enrollment := range m.massageEnrollments {
		if enrollment.CycleID == id {
			enrollmentIDs[enrollment.ID] = true
			delete(m.massageEnrollments, key)
		}
	}
	for callID, call := range m.massageCalls {
		if sessionIDs[call.SessionID] || enrollmentIDs[call.EnrollmentID] {
			delete(m.massageCalls, callID)
		}
	}
	for deliveryID, delivery := range m.massageDeliveries {
		if delivery.CycleID == id {
			delete(m.massageDeliveries, deliveryID)
		}
	}
	return nil
}
func (m *Memory) PublishMassageCycle(_ context.Context, id string, userIDs []string, deliveries []domain.MassageDelivery, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.massageCycles[id]
	if !ok {
		return ErrNotFound
	}
	if v.Status != "draft" {
		return ErrConflict
	}
	v.Status = "scheduled"
	v.UpdatedAt = now
	m.massageCycles[id] = v
	m.massageEligible[id] = map[string]bool{}
	for _, u := range userIDs {
		m.massageEligible[id][u] = true
	}
	for _, d := range deliveries {
		m.massageDeliveries[d.ID] = d
	}
	return nil
}
func (m *Memory) ListMassageEligibleUserIDs(_ context.Context, cycleID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []string{}
	for id := range m.massageEligible[cycleID] {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}
func (m *Memory) MassageResponse(_ context.Context, cycleID, userID, action string, now time.Time) (domain.MassageEnrollment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cycle, ok := m.massageCycles[cycleID]
	if !ok {
		return domain.MassageEnrollment{}, ErrNotFound
	}
	if cycle.Status != "signup_open" || now.After(cycle.SignupDeadline) {
		return domain.MassageEnrollment{}, ErrConflict
	}
	if !m.massageEligible[cycleID][userID] {
		return domain.MassageEnrollment{}, ErrForbidden
	}
	key := massageEnrollmentKey(cycleID, userID)
	v := m.massageEnrollments[key]
	if v.ID == "" {
		v = domain.MassageEnrollment{ID: ids.New("men"), CycleID: cycleID, UserID: userID, CreatedAt: now}
	}
	switch action {
	case "enroll":
		if v.Status != "enrolled" {
			cycle.NextQueue++
			v.QueueNumber = cycle.NextQueue
			v.Status = "enrolled"
			v.EnrolledAt = &now
			v.WithdrawnAt = nil
		}
	case "decline":
		v.Status = "declined"
		v.QueueNumber = 0
		v.EnrolledAt = nil
	case "withdraw":
		if v.Status != "enrolled" || !now.Before(cycle.Sessions[0].StartsAt) {
			return v, ErrConflict
		}
		v.Status = "withdrawn"
		v.WithdrawnAt = &now
	default:
		return v, fmt.Errorf("invalid massage response")
	}
	v.UpdatedAt = now
	m.massageCycles[cycleID] = cycle
	m.massageEnrollments[key] = v
	return v, nil
}
func (m *Memory) ListMassageMe(_ context.Context, userID string) ([]domain.MassageMe, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []domain.MassageMe{}
	for _, cycle := range m.massageCycles {
		if !m.massageEligible[cycle.ID][userID] || cycle.Status == "completed" || cycle.Status == "cancelled" {
			continue
		}
		item := domain.MassageMe{Cycle: cycle}
		if e, ok := m.massageEnrollments[massageEnrollmentKey(cycle.ID, userID)]; ok {
			copy := e
			copy.UserName = m.users[userID].Name
			item.Enrollment = &copy
			for _, call := range m.massageCalls {
				if call.EnrollmentID == e.ID {
					c := call
					c.UserName = copy.UserName
					item.CurrentCall = &c
				}
			}
		}
		for _, call := range m.massageCalls {
			if call.Status == "awaiting_response" || call.Status == "accepted" {
				e := m.enrollmentByID(call.EnrollmentID)
				if e.CycleID == cycle.ID && (item.CurrentQueueNumber == 0 || e.QueueNumber < item.CurrentQueueNumber) {
					item.CurrentQueueNumber = e.QueueNumber
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}
func (m *Memory) enrollmentByID(id string) domain.MassageEnrollment {
	for _, e := range m.massageEnrollments {
		if e.ID == id {
			return e
		}
	}
	return domain.MassageEnrollment{}
}
func (m *Memory) CreateMassageResendDeliveries(_ context.Context, cycleID string, values []domain.MassageDelivery) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, d := range values {
		if !m.massageEligible[cycleID][d.UserID] {
			continue
		}
		e := m.massageEnrollments[massageEnrollmentKey(cycleID, d.UserID)]
		if e.Status == "enrolled" {
			continue
		}
		m.massageDeliveries[d.ID] = d
		count++
	}
	return count, nil
}
func (m *Memory) SetMassageSessionStatus(_ context.Context, id, status, reason string, now time.Time) (domain.MassageSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for cycleID, cycle := range m.massageCycles {
		for i, s := range cycle.Sessions {
			if s.ID != id {
				continue
			}
			valid := (status == "running" && (s.Status == "scheduled" || s.Status == "paused")) || (status == "paused" && s.Status == "running") || (status == "closed" && (s.Status == "running" || s.Status == "paused"))
			if !valid {
				return s, ErrConflict
			}
			s.Status = status
			s.UpdatedAt = now
			if status == "running" {
				if s.StartedAt == nil {
					s.StartedAt = &now
				}
				cycle.Status = "in_progress"
			}
			if status == "closed" {
				s.ClosedAt = &now
				s.CloseReason = reason
				if s.Sequence == 2 {
					cycle.Status = "completed"
				}
			}
			cycle.Sessions[i] = s
			cycle.UpdatedAt = now
			m.massageCycles[cycleID] = cycle
			return s, nil
		}
	}
	return domain.MassageSession{}, ErrNotFound
}
func (m *Memory) FillMassageSession(_ context.Context, sessionID string, now time.Time) ([]domain.MassageCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var cycle domain.MassageCycle
	var session domain.MassageSession
	for _, c := range m.massageCycles {
		for _, s := range c.Sessions {
			if s.ID == sessionID {
				cycle = c
				session = s
			}
		}
	}
	if session.ID == "" {
		return nil, ErrNotFound
	}
	if session.Status != "running" {
		return []domain.MassageCall{}, nil
	}
	active := 0
	used := map[string]bool{}
	for _, call := range m.massageCalls {
		if call.SessionID == sessionID {
			used[call.EnrollmentID] = true
			if call.Status == "pending_delivery" || call.Status == "awaiting_response" || call.Status == "accepted" {
				active++
			}
		}
	}
	slots := session.ConcurrentSlots - active
	if remaining := session.Quota - session.CompletedCount - active; remaining < slots {
		slots = remaining
	}
	candidates := []domain.MassageEnrollment{}
	for _, e := range m.massageEnrollments {
		if e.CycleID == cycle.ID && e.Status == "enrolled" && !used[e.ID] {
			candidates = append(candidates, e)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].QueueNumber < candidates[j].QueueNumber })
	out := []domain.MassageCall{}
	for i := 0; i < slots && i < len(candidates); i++ {
		e := candidates[i]
		call := domain.MassageCall{ID: ids.New("mcl"), SessionID: sessionID, EnrollmentID: e.ID, UserID: e.UserID, QueueNumber: e.QueueNumber, Status: "pending_delivery", CreatedAt: now, UpdatedAt: now}
		m.massageCalls[call.ID] = call
		d := domain.MassageDelivery{ID: ids.New("mdl"), CycleID: cycle.ID, SessionID: sessionID, CallID: call.ID, UserID: e.UserID, Kind: "call", Status: "pending", AvailableAt: now, IdempotencyKey: shortIdempotency("massage-call-" + call.ID), CreatedAt: now, UpdatedAt: now}
		m.massageDeliveries[d.ID] = d
		out = append(out, call)
	}
	return out, nil
}
func (m *Memory) ClaimMassageDeliveries(_ context.Context, now time.Time, lease time.Duration, limit int) ([]domain.MassageDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.massageCycles {
		if c.Status == "scheduled" && !now.Before(c.SignupNoticeAt) && now.Before(c.SignupDeadline) {
			c.Status = "signup_open"
			m.massageCycles[id] = c
		}
	}
	out := []domain.MassageDelivery{}
	for id, d := range m.massageDeliveries {
		if len(out) >= limit {
			break
		}
		if (d.Status == "pending" || d.Status == "retry" || d.Status == "sending") && !d.AvailableAt.After(now) && (d.LockedUntil == nil || d.LockedUntil.Before(now)) {
			d.Status = "sending"
			d.Attempts++
			until := now.Add(lease)
			d.LockedUntil = &until
			d.UpdatedAt = now
			m.massageDeliveries[id] = d
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *Memory) UpdateMassageDelivery(_ context.Context, v domain.MassageDelivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.massageDeliveries[v.ID]; !ok {
		return ErrNotFound
	}
	m.massageDeliveries[v.ID] = v
	return nil
}
func (m *Memory) MarkMassageCallSent(_ context.Context, id, messageID string, called, due time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.massageCalls[id]
	if !ok {
		return ErrNotFound
	}
	if v.Status == "pending_delivery" {
		v.Status = "awaiting_response"
		v.MessageID = messageID
		v.CalledAt = &called
		v.ResponseDueAt = &due
		v.UpdatedAt = called
		m.massageCalls[id] = v
	}
	return nil
}
func (m *Memory) MarkMassageCallDeliveryFailed(_ context.Context, id, message string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.massageCalls[id]
	if !ok {
		return ErrNotFound
	}
	v.Status = "delivery_failed"
	v.LastError = message
	v.UpdatedAt = now
	m.massageCalls[id] = v
	return nil
}
func (m *Memory) ExpireMassageCalls(_ context.Context, now time.Time) ([]domain.MassageCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.MassageCall{}
	for id, v := range m.massageCalls {
		if v.Status == "awaiting_response" && v.ResponseDueAt != nil && v.ResponseDueAt.Before(now) {
			v.Status = "timed_out"
			v.UpdatedAt = now
			m.massageCalls[id] = v
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *Memory) RespondMassageCall(_ context.Context, id, userID, action string, now time.Time) (domain.MassageCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.massageCalls[id]
	if !ok {
		return v, ErrNotFound
	}
	if v.UserID != userID || v.Status != "awaiting_response" || v.ResponseDueAt == nil || v.ResponseDueAt.Before(now) {
		return v, ErrConflict
	}
	if action == "accept" {
		v.Status = "accepted"
	} else if action == "reject" {
		v.Status = "rejected"
	} else {
		return v, fmt.Errorf("invalid call response")
	}
	v.RespondedAt = &now
	v.UpdatedAt = now
	m.massageCalls[id] = v
	return v, nil
}
func (m *Memory) CompleteMassageCall(_ context.Context, id, action string, now time.Time) (domain.MassageCall, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.massageCalls[id]
	if !ok {
		return v, ErrNotFound
	}
	if v.Status != "accepted" {
		return v, ErrConflict
	}
	if action == "complete" {
		v.Status = "completed"
		v.CompletedAt = &now
	} else if action == "no_show" {
		v.Status = "no_show"
	} else {
		return v, fmt.Errorf("invalid action")
	}
	v.UpdatedAt = now
	m.massageCalls[id] = v
	for cid, c := range m.massageCycles {
		for i, s := range c.Sessions {
			if s.ID == v.SessionID && action == "complete" {
				s.CompletedCount++
				if s.CompletedCount >= s.Quota {
					s.Status = "completed"
					s.ClosedAt = &now
				}
				c.Sessions[i] = s
				e := m.enrollmentByID(v.EnrollmentID)
				e.Status = "completed"
				e.UpdatedAt = now
				m.massageEnrollments[massageEnrollmentKey(e.CycleID, e.UserID)] = e
				if s.Sequence == 2 && s.Status == "completed" {
					c.Status = "completed"
				}
				m.massageCycles[cid] = c
			}
		}
	}
	return v, nil
}
func (m *Memory) GetMassageCall(_ context.Context, id string) (domain.MassageCall, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.massageCalls[id]
	if !ok {
		return v, ErrNotFound
	}
	e := m.enrollmentByID(v.EnrollmentID)
	v.UserID = e.UserID
	v.UserName = m.users[e.UserID].Name
	v.QueueNumber = e.QueueNumber
	return v, nil
}
func (m *Memory) MassageStatistics(_ context.Context, cycleID string) (domain.MassageStatistic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r := domain.MassageStatistic{CycleID: cycleID, Eligible: len(m.massageEligible[cycleID])}
	for _, e := range m.massageEnrollments {
		if e.CycleID != cycleID {
			continue
		}
		e.UserName = m.users[e.UserID].Name
		r.Enrollments = append(r.Enrollments, e)
		if e.QueueNumber > 0 {
			r.Enrolled++
		}
		if e.Status == "unserved" {
			r.Unserved++
		}
	}
	for _, c := range m.massageCalls {
		e := m.enrollmentByID(c.EnrollmentID)
		if e.CycleID != cycleID {
			continue
		}
		c.UserID = e.UserID
		c.UserName = m.users[e.UserID].Name
		c.QueueNumber = e.QueueNumber
		r.Calls = append(r.Calls, c)
		switch c.Status {
		case "completed":
			r.Completed++
		case "rejected":
			r.Rejected++
		case "timed_out":
			r.TimedOut++
		case "no_show":
			r.NoShow++
		case "delivery_failed":
			r.DeliveryFail++
		}
	}
	return r, nil
}
func (m *Memory) CleanupMassageHistory(context.Context, time.Time) error { return nil }
