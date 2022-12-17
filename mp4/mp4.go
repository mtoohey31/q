package mp4

import (
	"io"

	"github.com/abema/go-mp4"
	"github.com/faiface/beep"
)

func Decode(rc io.ReadCloser) (s beep.StreamSeekCloser, format beep.Format, err error) {

}
