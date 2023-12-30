package jwks

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_keychanged(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		current string
		want    bool
		wantErr bool
	}{
		{name: "missing file", data: []byte{1, 2, 3}, current: "missing.pem", want: true, wantErr: false},
		{name: "should equal", data: []byte("This is used to test jwks.keychanged."), current: filepath.Join("..", "..", "..", "testdata", "testfile.pem"), want: false, wantErr: false},
		{name: "should be different", data: []byte("This is some different text."), current: filepath.Join("..", "..", "..", "testdata", "testfile.pem"), want: true, wantErr: false},
	}
	for _, tt := range tests {
		got, err := keychanged(tt.current, tt.data)
		if tt.wantErr {
			assert.NotNil(t, err, tt.name+": err != nil")
			continue
		}

		assert.Nil(t, err, tt.name+": err == nil")
		assert.Equal(t, tt.want, got, tt.name+": tt.want == got")
	}
}
