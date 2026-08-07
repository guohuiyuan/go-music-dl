package soda

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// DecryptAudio 核心解密函数
func DecryptAudio(fileData []byte, playAuth string) ([]byte, error) {
	hexKey, err := extractKey(playAuth)
	if err != nil {
		return nil, err
	}
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}

	moov, err := findBox(fileData, "moov", 0, len(fileData))
	if err != nil {
		return nil, errors.New("moov box not found")
	}

	stbl, err := findBox(fileData, "stbl", moov.offset, moov.offset+moov.size)
	if err != nil {
		trak, _ := findBox(fileData, "trak", moov.offset+8, moov.offset+moov.size)
		if trak != nil {
			mdia, _ := findBox(fileData, "mdia", trak.offset+8, trak.offset+trak.size)
			if mdia != nil {
				minf, _ := findBox(fileData, "minf", mdia.offset+8, mdia.offset+mdia.size)
				if minf != nil {
					stbl, _ = findBox(fileData, "stbl", minf.offset+8, minf.offset+minf.size)
				}
			}
		}
	}
	if stbl == nil {
		return nil, errors.New("stbl box not found")
	}

	stsz, err := findBox(fileData, "stsz", stbl.offset+8, stbl.offset+stbl.size)
	if err != nil {
		return nil, errors.New("stsz box not found")
	}
	sampleSizes := parseStsz(stsz.data)

	senc, err := findBox(fileData, "senc", moov.offset+8, moov.offset+moov.size)
	if err != nil {
		senc, err = findBox(fileData, "senc", stbl.offset+8, stbl.offset+stbl.size)
	}
	if err != nil {
		return nil, errors.New("senc box not found")
	}
	sencSamples := parseSenc(senc.data, defaultPerSampleIVSize(fileData, stbl.offset, stbl.offset+stbl.size))

	mdat, err := findBox(fileData, "mdat", 0, len(fileData))
	if err != nil {
		return nil, errors.New("mdat box not found")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	decryptedData := make([]byte, len(fileData))
	copy(decryptedData, fileData)

	readPtr := mdat.offset + 8
	decryptedMdat := make([]byte, 0, mdat.size-8)

	for i := 0; i < len(sampleSizes); i++ {
		size := int(sampleSizes[i])
		if readPtr+size > len(decryptedData) {
			break
		}
		chunk := decryptedData[readPtr : readPtr+size]

		if i < len(sencSamples) {
			dst := decryptSencSample(block, chunk, sencSamples[i])
			decryptedMdat = append(decryptedMdat, dst...)
		} else {
			decryptedMdat = append(decryptedMdat, chunk...)
		}
		readPtr += size
	}

	if len(decryptedMdat) == int(mdat.size)-8 {
		copy(decryptedData[mdat.offset+8:], decryptedMdat)
	} else {
		return nil, errors.New("decrypted size mismatch")
	}

	stsd, err := findBox(fileData, "stsd", stbl.offset+8, stbl.offset+stbl.size)
	if err == nil {
		stsdOffset := stsd.offset
		stsdData := decryptedData[stsdOffset : stsdOffset+stsd.size]
		if idx := bytes.Index(stsdData, []byte("enca")); idx != -1 {
			copy(stsdData[idx:], encryptedSampleOriginalFormat(stsdData))
			copy(decryptedData[stsdOffset:], stsdData)
		}
	}

	return decryptedData, nil
}

func encryptedSampleOriginalFormat(stsdData []byte) []byte {
	idx := bytes.Index(stsdData, []byte("frma"))
	if idx < 4 || idx+8 > len(stsdData) {
		return []byte("mp4a")
	}
	size := int(binary.BigEndian.Uint32(stsdData[idx-4 : idx]))
	if size < 12 || idx-4+size > len(stsdData) {
		return []byte("mp4a")
	}
	return stsdData[idx+4 : idx+8]
}

func defaultPerSampleIVSize(data []byte, start, end int) int {
	tenc, err := findBoxDeep(data, "tenc", start, end)
	if err != nil || len(tenc.data) < 8 {
		return 8
	}
	ivSize := int(tenc.data[7])
	if ivSize == 8 || ivSize == 16 {
		return ivSize
	}
	return 8
}

type sodaSencSubsample struct {
	clear     uint16
	encrypted uint32
}

type sodaSencSample struct {
	iv         []byte
	subsamples []sodaSencSubsample
}

func decryptSencSample(block cipher.Block, chunk []byte, sample sodaSencSample) []byte {
	iv := sample.iv
	if len(iv) < aes.BlockSize {
		padded := make([]byte, aes.BlockSize)
		copy(padded, iv)
		iv = padded
	}
	stream := cipher.NewCTR(block, iv)

	if len(sample.subsamples) == 0 {
		dst := make([]byte, len(chunk))
		stream.XORKeyStream(dst, chunk)
		return dst
	}

	dst := make([]byte, len(chunk))
	pos := 0
	for _, sub := range sample.subsamples {
		clearBytes := int(sub.clear)
		if clearBytes > len(chunk)-pos {
			clearBytes = len(chunk) - pos
		}
		copy(dst[pos:pos+clearBytes], chunk[pos:pos+clearBytes])
		pos += clearBytes
		if pos >= len(chunk) {
			break
		}

		encryptedBytes := int(sub.encrypted)
		if encryptedBytes > len(chunk)-pos {
			encryptedBytes = len(chunk) - pos
		}
		stream.XORKeyStream(dst[pos:pos+encryptedBytes], chunk[pos:pos+encryptedBytes])
		pos += encryptedBytes
		if pos >= len(chunk) {
			break
		}
	}
	if pos < len(chunk) {
		copy(dst[pos:], chunk[pos:])
	}
	return dst
}

type mp4Box struct {
	offset int
	size   int
	data   []byte
}

func findBox(data []byte, boxType string, start, end int) (*mp4Box, error) {
	if end > len(data) {
		end = len(data)
	}
	pos := start
	target := []byte(boxType)
	for pos+8 <= end {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		if size < 8 {
			break
		}
		if bytes.Equal(data[pos+4:pos+8], target) {
			return &mp4Box{offset: pos, size: size, data: data[pos+8 : pos+size]}, nil
		}
		pos += size
	}
	return nil, errors.New("box not found")
}

func findBoxDeep(data []byte, boxType string, start, end int) (*mp4Box, error) {
	if end > len(data) {
		end = len(data)
	}
	pos := start
	target := []byte(boxType)
	for pos+8 <= end {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		headerSize := 8
		if size == 1 {
			if pos+16 > end {
				break
			}
			size64 := binary.BigEndian.Uint64(data[pos+8 : pos+16])
			if size64 > uint64(end-pos) {
				break
			}
			size = int(size64)
			headerSize = 16
		}
		if size < headerSize || pos+size > end {
			break
		}
		currentType := string(data[pos+4 : pos+8])
		if bytes.Equal(data[pos+4:pos+8], target) {
			return &mp4Box{offset: pos, size: size, data: data[pos+headerSize : pos+size]}, nil
		}

		if childStart, ok := boxChildStart(currentType, pos, headerSize); ok && childStart < pos+size {
			if found, err := findBoxDeep(data, boxType, childStart, pos+size); err == nil {
				return found, nil
			}
		}
		pos += size
	}
	return nil, errors.New("box not found")
}

func boxChildStart(boxType string, offset, headerSize int) (int, bool) {
	switch boxType {
	case "moov", "trak", "mdia", "minf", "stbl", "sinf", "schi":
		return offset + headerSize, true
	case "stsd":
		return offset + headerSize + 8, true
	case "enca", "mp4a", "alac", "fLaC":
		return offset + headerSize + 28, true
	default:
		return 0, false
	}
}

func parseStsz(data []byte) []uint32 {
	if len(data) < 12 {
		return nil
	}
	sampleSizeFixed := binary.BigEndian.Uint32(data[4:8])
	sampleCount := int(binary.BigEndian.Uint32(data[8:12]))
	sizes := make([]uint32, sampleCount)
	if sampleSizeFixed != 0 {
		for i := 0; i < sampleCount; i++ {
			sizes[i] = sampleSizeFixed
		}
	} else {
		for i := 0; i < sampleCount; i++ {
			if 12+i*4+4 <= len(data) {
				sizes[i] = binary.BigEndian.Uint32(data[12+i*4 : 12+i*4+4])
			}
		}
	}
	return sizes
}

func parseSenc(data []byte, ivSize int) []sodaSencSample {
	if len(data) < 8 {
		return nil
	}
	if ivSize != 8 && ivSize != 16 {
		ivSize = 8
	}
	flags := binary.BigEndian.Uint32(data[0:4]) & 0x00FFFFFF
	sampleCount := int(binary.BigEndian.Uint32(data[4:8]))
	samples := make([]sodaSencSample, 0, sampleCount)
	ptr := 8
	hasSubsamples := (flags & 0x02) != 0
	for i := 0; i < sampleCount; i++ {
		if ptr+ivSize > len(data) {
			break
		}
		sample := sodaSencSample{
			iv: append([]byte(nil), data[ptr:ptr+ivSize]...),
		}
		ptr += ivSize
		if hasSubsamples {
			if ptr+2 > len(data) {
				break
			}
			subCount := int(binary.BigEndian.Uint16(data[ptr : ptr+2]))
			ptr += 2
			if ptr+subCount*6 > len(data) {
				break
			}
			sample.subsamples = make([]sodaSencSubsample, 0, subCount)
			for j := 0; j < subCount; j++ {
				sample.subsamples = append(sample.subsamples, sodaSencSubsample{
					clear:     binary.BigEndian.Uint16(data[ptr : ptr+2]),
					encrypted: binary.BigEndian.Uint32(data[ptr+2 : ptr+6]),
				})
				ptr += 6
			}
		}
		samples = append(samples, sample)
	}
	return samples
}

func bitcount(n int) int {
	u := uint32(n)
	u = u & 0xFFFFFFFF
	u = u - ((u >> 1) & 0x55555555)
	u = (u & 0x33333333) + ((u >> 2) & 0x33333333)
	return int((((u + (u >> 4)) & 0xF0F0F0F) * 0x1010101) >> 24)
}

func decodeBase36(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'z' {
		return int(c - 'a' + 10)
	}
	return 0xFF
}

func decryptSpadeInner(keyBytes []byte) []byte {
	result := make([]byte, len(keyBytes))
	buff := append([]byte{0xFA, 0x55}, keyBytes...)
	for i := 0; i < len(result); i++ {
		v := int(keyBytes[i]^buff[i]) - bitcount(i) - 21
		for v < 0 {
			v += 255
		}
		result[i] = byte(v)
	}
	return result
}

func extractKey(playAuth string) (string, error) {
	binaryStr, err := base64.StdEncoding.DecodeString(playAuth)
	if err != nil {
		return "", err
	}
	bytesData := []byte(binaryStr)
	if len(bytesData) < 3 {
		return "", errors.New("auth data too short")
	}
	paddingLen := int((bytesData[0] ^ bytesData[1] ^ bytesData[2]) - 48)
	if len(bytesData) < paddingLen+2 {
		return "", errors.New("invalid padding length")
	}
	innerInput := bytesData[1 : len(bytesData)-paddingLen]
	tmpBuff := decryptSpadeInner(innerInput)
	if len(tmpBuff) == 0 {
		return "", errors.New("decryption failed")
	}
	skipBytes := decodeBase36(tmpBuff[0])
	endIndex := 1 + (len(bytesData) - paddingLen - 2) - skipBytes
	if endIndex > len(tmpBuff) || endIndex < 1 {
		return "", errors.New("index out of bounds")
	}
	return string(tmpBuff[1:endIndex]), nil
}
