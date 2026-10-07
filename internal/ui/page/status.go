package page

// NoteOf and ErrorOf are the optional status channels shared by nested pages
// and the application chrome.
func NoteOf(component any) string {
	if source, ok := component.(interface{ Note() string }); ok {
		return source.Note()
	}

	return ""
}

func ErrorOf(component any) string {
	if source, ok := component.(interface{ Error() string }); ok {
		return source.Error()
	}

	return ""
}
