package qq

import (
	"bytes"
	"errors"
)

var defaultQQMask58 = []byte{
	74, 214, 202, 144, 103, 247, 82,
	94, 149, 35, 159, 19, 17, 126,
	71, 116, 61, 144, 170, 63, 81,
	198, 9, 213, 159, 250, 102, 249,
	243, 214, 161, 144, 160, 247, 240,
	29, 149, 222, 159, 132, 17, 244,
	14, 116, 187, 144, 188, 63, 146,
	0, 9, 91, 159, 98, 102, 161,
}

const (
	defaultQQSuper58A byte = 195
	defaultQQSuper58B byte = 216
)

func DecryptQQ(encrypted []byte, ext string) ([]byte, string, error) {
	if len(encrypted) == 0 {
		return nil, "", errors.New("empty input")
	}

	mask := newQQMask(defaultQQMask58, defaultQQSuper58A, defaultQQSuper58B)
	if ext == "mflac" {
		if detected := detectQQMaskFromEncrypted(encrypted); detected != nil {
			mask = detected
		}
	}

	plain := mask.Decrypt(encrypted)

	switch ext {
	case "mflac", "qmcflac", "bkcflac":
		return plain, "flac", nil
	case "mgg", "qmcogg":
		return plain, "ogg", nil
	case "tkm":
		return plain, "m4a", nil
	default:
		if extGuess := detectAudioExt(plain); extGuess != "" {
			return plain, extGuess, nil
		}
		return plain, "mp3", nil
	}
}

func detectAudioExt(data []byte) string {
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'f', 'L', 'a', 'C'}) {
		return "flac"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{'I', 'D', '3'}) {
		return "mp3"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'O', 'g', 'g', 'S'}) {
		return "ogg"
	}
	if len(data) >= 8 && bytes.Equal(data[4:8], []byte{'f', 't', 'y', 'p'}) {
		return "m4a"
	}
	return "mp3"
}

type qqMask struct {
	matrix128 [128]byte
	matrix58  []byte
	superA    byte
	superB    byte
}

func newQQMask(matrix58 []byte, superA, superB byte) *qqMask {
	m := &qqMask{matrix58: append([]byte(nil), matrix58...), superA: superA, superB: superB}
	m.generateMask128From58()
	return m
}

func newQQMaskFrom128(matrix128 []byte) (*qqMask, error) {
	if len(matrix128) != 128 {
		return nil, errors.New("incorrect mask128 length")
	}
	m := &qqMask{}
	copy(m.matrix128[:], matrix128)
	if err := m.generateMask58From128(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *qqMask) generateMask128From58() {
	var out [128]byte
	idx := 0
	for i := 0; i < 8; i++ {
		out[idx] = m.superA
		idx++
		chunk := m.matrix58[7*i : 7*i+7]
		copy(out[idx:idx+7], chunk)
		idx += 7
		out[idx] = m.superB
		idx++
		revChunk := m.matrix58[49-7*i : 56-7*i]
		for j := len(revChunk) - 1; j >= 0; j-- {
			out[idx] = revChunk[j]
			idx++
		}
	}
	m.matrix128 = out
}

func (m *qqMask) generateMask58From128() error {
	e := m.matrix128[0]
	b := m.matrix128[8]
	out := make([]byte, 0, 56)

	for n := 0; n < 8; n++ {
		i := 16 * n
		o := 120 - i
		if m.matrix128[i] != e || m.matrix128[i+8] != b {
			return errors.New("decode mask-128 to mask-58 failed")
		}

		a := m.matrix128[i+1 : i+8]
		c := make([]byte, 7)
		for j := 0; j < 7; j++ {
			c[j] = m.matrix128[o+7-j]
		}
		if !bytes.Equal(a, c) {
			return errors.New("decode mask-128 to mask-58 failed")
		}
		out = append(out, a...)
	}

	m.matrix58 = out
	m.superA = e
	m.superB = b
	return nil
}

func (m *qqMask) Decrypt(encrypted []byte) []byte {
	out := append([]byte(nil), encrypted...)
	r := -1
	n := -1
	for i := 0; i < len(out); i++ {
		r++
		n++
		if r == 32768 || (r > 32768 && (r+1)%32768 == 0) {
			r++
			n++
		}
		if n >= 128 {
			n -= 128
		}
		out[i] ^= m.matrix128[n]
	}
	return out
}

func detectQQMaskFromEncrypted(encrypted []byte) *qqMask {
	max := len(encrypted)
	if max > 32768 {
		max = 32768
	}

	for i := 0; i+128 <= max; i += 128 {
		mask, err := newQQMaskFrom128(encrypted[i : i+128])
		if err != nil {
			continue
		}
		head := mask.Decrypt(encrypted[:4])
		if bytes.Equal(head, []byte{'f', 'L', 'a', 'C'}) {
			return mask
		}
	}

	return nil
}
