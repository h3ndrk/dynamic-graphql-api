package schema

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type cursor struct {
	object string
	id     uint
}

func parseCursor(c string) (*cursor, error) {
	bytesCursor, err := base64.StdEncoding.DecodeString(c)
	if err != nil {
		return nil, errors.Errorf("Invalid cursor '%s'", c)
	}

	stringsID := strings.SplitN(string(bytesCursor), ":", 2)
	if len(stringsID) != 2 {
		return nil, errors.Errorf("Invalid cursor '%s'", c)
	}

	int64ID, err := strconv.ParseInt(stringsID[1], 10, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "Invalid cursor '%s'", c)
	}

	return &cursor{object: stringsID[0], id: uint(int64ID)}, nil
}

func (c cursor) String() string {
	return fmt.Sprintf("%s:%d", c.object, c.id)
}

func (c cursor) OpaqueString() string {
	return base64.StdEncoding.EncodeToString([]byte(c.String()))
}
