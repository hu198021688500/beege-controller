package docker

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
)

func GenerateRandomID() string {
	for {
		id := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, id); err != nil {
			panic(err) // This shouldn't happen
		}
		value := hex.EncodeToString(id)
		// if we try to parse the truncated for as an int and we don't have
		// an error then the value is all numberic and causes issues when
		// used as a hostname. ref #3869
		if _, err := strconv.Atoi(TruncateID(value)); err == nil {
			continue
		}
		return value
	}
}

func TruncateID(id string) string {
	shortLen := 32
	if len(id) < shortLen {
		shortLen = len(id)
	}
	return id[:shortLen]
}

func GenerateUUID() string {
	id := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		panic(err)
	}
	value := hex.EncodeToString(id)
	return fmt.Sprintf("%s-%s-%s-%s-%s", value[0:8], value[8:12], value[12:16],
		value[16:20], value[20:32])
}
