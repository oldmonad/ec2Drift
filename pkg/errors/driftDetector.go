package errors

type ErrDriftDetected struct {
	Message string
}

func (e ErrDriftDetected) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "drift detected"
}

func NewDriftDetected() error {
	return ErrDriftDetected{
		Message: "drift detected",
	}
}
