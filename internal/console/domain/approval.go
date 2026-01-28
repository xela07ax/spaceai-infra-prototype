package domain

type ApprovalRequest struct {
	ID          uuid.UUID
	ExecutionID uuid.UUID // Ссылка на зависший запрос в UAG
	Payload     []byte    // Что именно хочет сделать агент
	Status      string    // pending, approved, rejected
	ReviewerID  uuid.UUID
	Comment     string
}
