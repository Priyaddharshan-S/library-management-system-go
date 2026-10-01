package models

type TopBook struct {
	BookID       int    `json:"book_id"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	BorrowCount  int    `json:"borrow_count"`
}

type ReportSummary struct {
	TotalBooks        int       `json:"total_books"`
	AvailableBooks    int       `json:"available_books"`
	ActiveLoans       int       `json:"active_loans"`
	OverdueLoans      int       `json:"overdue_loans"`
	TopBorrowedBooks  []TopBook `json:"top_borrowed_books"`
}
