package news

import (
	"html"
	"io"
)

func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	lr := io.LimitReader(r, limit)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) >= limit {
		return nil, errTooLarge
	}
	return b, nil
}

var errTooLarge = &tooLargeError{}

type tooLargeError struct{}

func (*tooLargeError) Error() string { return "response exceeded size limit" }

func decodeEntities(s string) string {
	return html.UnescapeString(s)
}
