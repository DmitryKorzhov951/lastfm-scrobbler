package lastfm

import "errors"

func asError(err error, target **Error) bool {
	return errors.As(err, target)
}

func AsError(err error) (*Error, bool) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
