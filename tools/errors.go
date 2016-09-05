package tools

type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	s := ""
	for _, e := range m.Errors {
		s += ", " + e.Error()
	}

	return s
}

func (m *MultiError) Append(err error) {
	if m.Errors == nil {
		m.Errors = []error{err}
	} else {
		m.Errors = append(m.Errors, err)
	}
}
