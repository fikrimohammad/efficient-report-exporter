package s3

import (
	"context"
	"errors"

	commons3 "github.com/fikrimohammad/go-dev-sdk/s3"
)

var (
	errUpload  = errors.New("upload failed")
	errPresign = errors.New("presign failed")
)

// stubClient is a stub commons3.Client that records the operations performed
// against it and can be configured to fail uploads or presigns.
type stubClient struct {
	uploadErr    error
	presignErr   error
	presignURL   string
	uploadCalls  int
	presignCalls int
	lastUpload   commons3.UploadObjectParams
	lastPresign  commons3.PresignGetObjectParams
}

func (s *stubClient) UploadObject(_ context.Context, params commons3.UploadObjectParams) error {
	s.uploadCalls++
	s.lastUpload = params
	return s.uploadErr
}

func (s *stubClient) PresignGetObject(_ context.Context, params commons3.PresignGetObjectParams) (string, error) {
	s.presignCalls++
	s.lastPresign = params
	if s.presignErr != nil {
		return "", s.presignErr
	}
	return s.presignURL, nil
}
