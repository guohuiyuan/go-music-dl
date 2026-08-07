package lyrics

var qrcSBox = [8][64]byte{
	{14, 4, 13, 1, 2, 15, 11, 8, 3, 10, 6, 12, 5, 9, 0, 7, 0, 15, 7, 4, 14, 2, 13, 1, 10, 6, 12, 11, 9, 5, 3, 8, 4, 1, 14, 8, 13, 6, 2, 11, 15, 12, 9, 7, 3, 10, 5, 0, 15, 12, 8, 2, 4, 9, 1, 7, 5, 11, 3, 14, 10, 0, 6, 13},
	{15, 1, 8, 14, 6, 11, 3, 4, 9, 7, 2, 13, 12, 0, 5, 10, 3, 13, 4, 7, 15, 2, 8, 15, 12, 0, 1, 10, 6, 9, 11, 5, 0, 14, 7, 11, 10, 4, 13, 1, 5, 8, 12, 6, 9, 3, 2, 15, 13, 8, 10, 1, 3, 15, 4, 2, 11, 6, 7, 12, 0, 5, 14, 9},
	{10, 0, 9, 14, 6, 3, 15, 5, 1, 13, 12, 7, 11, 4, 2, 8, 13, 7, 0, 9, 3, 4, 6, 10, 2, 8, 5, 14, 12, 11, 15, 1, 13, 6, 4, 9, 8, 15, 3, 0, 11, 1, 2, 12, 5, 10, 14, 7, 1, 10, 13, 0, 6, 9, 8, 7, 4, 15, 14, 3, 11, 5, 2, 12},
	{7, 13, 14, 3, 0, 6, 9, 10, 1, 2, 8, 5, 11, 12, 4, 15, 13, 8, 11, 5, 6, 15, 0, 3, 4, 7, 2, 12, 1, 10, 14, 9, 10, 6, 9, 0, 12, 11, 7, 13, 15, 1, 3, 14, 5, 2, 8, 4, 3, 15, 0, 6, 10, 10, 13, 8, 9, 4, 5, 11, 12, 7, 2, 14},
	{2, 12, 4, 1, 7, 10, 11, 6, 8, 5, 3, 15, 13, 0, 14, 9, 14, 11, 2, 12, 4, 7, 13, 1, 5, 0, 15, 10, 3, 9, 8, 6, 4, 2, 1, 11, 10, 13, 7, 8, 15, 9, 12, 5, 6, 3, 0, 14, 11, 8, 12, 7, 1, 14, 2, 13, 6, 15, 0, 9, 10, 4, 5, 3},
	{12, 1, 10, 15, 9, 2, 6, 8, 0, 13, 3, 4, 14, 7, 5, 11, 10, 15, 4, 2, 7, 12, 9, 5, 6, 1, 13, 14, 0, 11, 3, 8, 9, 14, 15, 5, 2, 8, 12, 3, 7, 0, 4, 10, 1, 13, 11, 6, 4, 3, 2, 12, 9, 5, 15, 10, 11, 14, 1, 7, 6, 0, 8, 13},
	{4, 11, 2, 14, 15, 0, 8, 13, 3, 12, 9, 7, 5, 10, 6, 1, 13, 0, 11, 7, 4, 9, 1, 10, 14, 3, 5, 12, 2, 15, 8, 6, 1, 4, 11, 13, 12, 3, 7, 14, 10, 15, 6, 8, 0, 5, 9, 2, 6, 11, 13, 8, 1, 4, 10, 7, 9, 5, 0, 15, 14, 2, 3, 12},
	{13, 2, 8, 4, 6, 15, 11, 1, 10, 9, 3, 14, 5, 0, 12, 7, 1, 15, 13, 8, 10, 3, 7, 4, 12, 5, 6, 11, 0, 14, 9, 2, 7, 11, 4, 1, 9, 12, 14, 2, 0, 6, 10, 13, 15, 3, 5, 8, 2, 1, 14, 7, 4, 10, 8, 13, 15, 12, 9, 0, 3, 5, 6, 11},
}

const (
	qrcEncrypt = 1
	qrcDecrypt = 0
)

func qrcTripleDESDecrypt(block []byte) []byte {
	keys := [3][16][6]byte{
		qrcKeySchedule(qrcKey[16:], qrcDecrypt),
		qrcKeySchedule(qrcKey[8:], qrcEncrypt),
		qrcKeySchedule(qrcKey[0:], qrcDecrypt),
	}
	data := append([]byte(nil), block...)
	for i := 0; i < 3; i++ {
		data = qrcDESCrypt(data, keys[i])
	}
	return data
}

func qrcBitNum(a []byte, b, c int) uint32 {
	return uint32((a[(b/32)*4+3-(b%32)/8]>>(7-b%8))&1) << c
}

func qrcBitNumIntr(a uint32, b, c int) uint32 {
	return ((a >> (31 - b)) & 1) << c
}

func qrcBitNumIntl(a uint32, b, c int) uint32 {
	return ((a << b) & 0x80000000) >> c
}

func qrcSBoxBit(a byte) byte {
	return (a & 32) | ((a & 31) >> 1) | ((a & 1) << 4)
}

func qrcInitialPermutation(input []byte) (uint32, uint32) {
	return qrcBitNum(input, 57, 31) | qrcBitNum(input, 49, 30) | qrcBitNum(input, 41, 29) | qrcBitNum(input, 33, 28) |
			qrcBitNum(input, 25, 27) | qrcBitNum(input, 17, 26) | qrcBitNum(input, 9, 25) | qrcBitNum(input, 1, 24) |
			qrcBitNum(input, 59, 23) | qrcBitNum(input, 51, 22) | qrcBitNum(input, 43, 21) | qrcBitNum(input, 35, 20) |
			qrcBitNum(input, 27, 19) | qrcBitNum(input, 19, 18) | qrcBitNum(input, 11, 17) | qrcBitNum(input, 3, 16) |
			qrcBitNum(input, 61, 15) | qrcBitNum(input, 53, 14) | qrcBitNum(input, 45, 13) | qrcBitNum(input, 37, 12) |
			qrcBitNum(input, 29, 11) | qrcBitNum(input, 21, 10) | qrcBitNum(input, 13, 9) | qrcBitNum(input, 5, 8) |
			qrcBitNum(input, 63, 7) | qrcBitNum(input, 55, 6) | qrcBitNum(input, 47, 5) | qrcBitNum(input, 39, 4) |
			qrcBitNum(input, 31, 3) | qrcBitNum(input, 23, 2) | qrcBitNum(input, 15, 1) | qrcBitNum(input, 7, 0),
		qrcBitNum(input, 56, 31) | qrcBitNum(input, 48, 30) | qrcBitNum(input, 40, 29) | qrcBitNum(input, 32, 28) |
			qrcBitNum(input, 24, 27) | qrcBitNum(input, 16, 26) | qrcBitNum(input, 8, 25) | qrcBitNum(input, 0, 24) |
			qrcBitNum(input, 58, 23) | qrcBitNum(input, 50, 22) | qrcBitNum(input, 42, 21) | qrcBitNum(input, 34, 20) |
			qrcBitNum(input, 26, 19) | qrcBitNum(input, 18, 18) | qrcBitNum(input, 10, 17) | qrcBitNum(input, 2, 16) |
			qrcBitNum(input, 60, 15) | qrcBitNum(input, 52, 14) | qrcBitNum(input, 44, 13) | qrcBitNum(input, 36, 12) |
			qrcBitNum(input, 28, 11) | qrcBitNum(input, 20, 10) | qrcBitNum(input, 12, 9) | qrcBitNum(input, 4, 8) |
			qrcBitNum(input, 62, 7) | qrcBitNum(input, 54, 6) | qrcBitNum(input, 46, 5) | qrcBitNum(input, 38, 4) |
			qrcBitNum(input, 30, 3) | qrcBitNum(input, 22, 2) | qrcBitNum(input, 14, 1) | qrcBitNum(input, 6, 0)
}

func qrcInversePermutation(s0, s1 uint32) []byte {
	data := make([]byte, 8)
	data[3] = byte(qrcBitNumIntr(s1, 7, 7) | qrcBitNumIntr(s0, 7, 6) | qrcBitNumIntr(s1, 15, 5) | qrcBitNumIntr(s0, 15, 4) | qrcBitNumIntr(s1, 23, 3) | qrcBitNumIntr(s0, 23, 2) | qrcBitNumIntr(s1, 31, 1) | qrcBitNumIntr(s0, 31, 0))
	data[2] = byte(qrcBitNumIntr(s1, 6, 7) | qrcBitNumIntr(s0, 6, 6) | qrcBitNumIntr(s1, 14, 5) | qrcBitNumIntr(s0, 14, 4) | qrcBitNumIntr(s1, 22, 3) | qrcBitNumIntr(s0, 22, 2) | qrcBitNumIntr(s1, 30, 1) | qrcBitNumIntr(s0, 30, 0))
	data[1] = byte(qrcBitNumIntr(s1, 5, 7) | qrcBitNumIntr(s0, 5, 6) | qrcBitNumIntr(s1, 13, 5) | qrcBitNumIntr(s0, 13, 4) | qrcBitNumIntr(s1, 21, 3) | qrcBitNumIntr(s0, 21, 2) | qrcBitNumIntr(s1, 29, 1) | qrcBitNumIntr(s0, 29, 0))
	data[0] = byte(qrcBitNumIntr(s1, 4, 7) | qrcBitNumIntr(s0, 4, 6) | qrcBitNumIntr(s1, 12, 5) | qrcBitNumIntr(s0, 12, 4) | qrcBitNumIntr(s1, 20, 3) | qrcBitNumIntr(s0, 20, 2) | qrcBitNumIntr(s1, 28, 1) | qrcBitNumIntr(s0, 28, 0))
	data[7] = byte(qrcBitNumIntr(s1, 3, 7) | qrcBitNumIntr(s0, 3, 6) | qrcBitNumIntr(s1, 11, 5) | qrcBitNumIntr(s0, 11, 4) | qrcBitNumIntr(s1, 19, 3) | qrcBitNumIntr(s0, 19, 2) | qrcBitNumIntr(s1, 27, 1) | qrcBitNumIntr(s0, 27, 0))
	data[6] = byte(qrcBitNumIntr(s1, 2, 7) | qrcBitNumIntr(s0, 2, 6) | qrcBitNumIntr(s1, 10, 5) | qrcBitNumIntr(s0, 10, 4) | qrcBitNumIntr(s1, 18, 3) | qrcBitNumIntr(s0, 18, 2) | qrcBitNumIntr(s1, 26, 1) | qrcBitNumIntr(s0, 26, 0))
	data[5] = byte(qrcBitNumIntr(s1, 1, 7) | qrcBitNumIntr(s0, 1, 6) | qrcBitNumIntr(s1, 9, 5) | qrcBitNumIntr(s0, 9, 4) | qrcBitNumIntr(s1, 17, 3) | qrcBitNumIntr(s0, 17, 2) | qrcBitNumIntr(s1, 25, 1) | qrcBitNumIntr(s0, 25, 0))
	data[4] = byte(qrcBitNumIntr(s1, 0, 7) | qrcBitNumIntr(s0, 0, 6) | qrcBitNumIntr(s1, 8, 5) | qrcBitNumIntr(s0, 8, 4) | qrcBitNumIntr(s1, 16, 3) | qrcBitNumIntr(s0, 16, 2) | qrcBitNumIntr(s1, 24, 1) | qrcBitNumIntr(s0, 24, 0))
	return data
}

func qrcF(state uint32, key [6]byte) uint32 {
	t1 := qrcBitNumIntl(state, 31, 0) | ((state & 0xf0000000) >> 1) | qrcBitNumIntl(state, 4, 5) |
		qrcBitNumIntl(state, 3, 6) | ((state & 0x0f000000) >> 3) | qrcBitNumIntl(state, 8, 11) |
		qrcBitNumIntl(state, 7, 12) | ((state & 0x00f00000) >> 5) | qrcBitNumIntl(state, 12, 17) |
		qrcBitNumIntl(state, 11, 18) | ((state & 0x000f0000) >> 7) | qrcBitNumIntl(state, 16, 23)
	t2 := qrcBitNumIntl(state, 15, 0) | ((state & 0x0000f000) << 15) | qrcBitNumIntl(state, 20, 5) |
		qrcBitNumIntl(state, 19, 6) | ((state & 0x00000f00) << 13) | qrcBitNumIntl(state, 24, 11) |
		qrcBitNumIntl(state, 23, 12) | ((state & 0x000000f0) << 11) | qrcBitNumIntl(state, 28, 17) |
		qrcBitNumIntl(state, 27, 18) | ((state & 0x0000000f) << 9) | qrcBitNumIntl(state, 0, 23)
	lrg := [6]byte{byte(t1 >> 24), byte(t1 >> 16), byte(t1 >> 8), byte(t2 >> 24), byte(t2 >> 16), byte(t2 >> 8)}
	for i := range lrg {
		lrg[i] ^= key[i]
	}
	state = uint32(qrcSBox[0][qrcSBoxBit(lrg[0]>>2)])<<28 |
		uint32(qrcSBox[1][qrcSBoxBit(((lrg[0]&0x03)<<4)|(lrg[1]>>4))])<<24 |
		uint32(qrcSBox[2][qrcSBoxBit(((lrg[1]&0x0f)<<2)|(lrg[2]>>6))])<<20 |
		uint32(qrcSBox[3][qrcSBoxBit(lrg[2]&0x3f)])<<16 |
		uint32(qrcSBox[4][qrcSBoxBit(lrg[3]>>2)])<<12 |
		uint32(qrcSBox[5][qrcSBoxBit(((lrg[3]&0x03)<<4)|(lrg[4]>>4))])<<8 |
		uint32(qrcSBox[6][qrcSBoxBit(((lrg[4]&0x0f)<<2)|(lrg[5]>>6))])<<4 |
		uint32(qrcSBox[7][qrcSBoxBit(lrg[5]&0x3f)])
	return qrcBitNumIntl(state, 15, 0) | qrcBitNumIntl(state, 6, 1) | qrcBitNumIntl(state, 19, 2) |
		qrcBitNumIntl(state, 20, 3) | qrcBitNumIntl(state, 28, 4) | qrcBitNumIntl(state, 11, 5) |
		qrcBitNumIntl(state, 27, 6) | qrcBitNumIntl(state, 16, 7) | qrcBitNumIntl(state, 0, 8) |
		qrcBitNumIntl(state, 14, 9) | qrcBitNumIntl(state, 22, 10) | qrcBitNumIntl(state, 25, 11) |
		qrcBitNumIntl(state, 4, 12) | qrcBitNumIntl(state, 17, 13) | qrcBitNumIntl(state, 30, 14) |
		qrcBitNumIntl(state, 9, 15) | qrcBitNumIntl(state, 1, 16) | qrcBitNumIntl(state, 7, 17) |
		qrcBitNumIntl(state, 23, 18) | qrcBitNumIntl(state, 13, 19) | qrcBitNumIntl(state, 31, 20) |
		qrcBitNumIntl(state, 26, 21) | qrcBitNumIntl(state, 2, 22) | qrcBitNumIntl(state, 8, 23) |
		qrcBitNumIntl(state, 18, 24) | qrcBitNumIntl(state, 12, 25) | qrcBitNumIntl(state, 29, 26) |
		qrcBitNumIntl(state, 5, 27) | qrcBitNumIntl(state, 21, 28) | qrcBitNumIntl(state, 10, 29) |
		qrcBitNumIntl(state, 3, 30) | qrcBitNumIntl(state, 24, 31)
}

func qrcDESCrypt(input []byte, key [16][6]byte) []byte {
	s0, s1 := qrcInitialPermutation(input)
	for i := 0; i < 15; i++ {
		prev := s1
		s1 = qrcF(s1, key[i]) ^ s0
		s0 = prev
	}
	s0 = qrcF(s1, key[15]) ^ s0
	return qrcInversePermutation(s0, s1)
}

func qrcKeySchedule(key []byte, mode int) [16][6]byte {
	var schedule [16][6]byte
	keyRndShift := [16]int{1, 1, 2, 2, 2, 2, 2, 2, 1, 2, 2, 2, 2, 2, 2, 1}
	keyPermC := [28]int{56, 48, 40, 32, 24, 16, 8, 0, 57, 49, 41, 33, 25, 17, 9, 1, 58, 50, 42, 34, 26, 18, 10, 2, 59, 51, 43, 35}
	keyPermD := [28]int{62, 54, 46, 38, 30, 22, 14, 6, 61, 53, 45, 37, 29, 21, 13, 5, 60, 52, 44, 36, 28, 20, 12, 4, 27, 19, 11, 3}
	keyCompression := [48]int{13, 16, 10, 23, 0, 4, 2, 27, 14, 5, 20, 9, 22, 18, 11, 3, 25, 7, 15, 6, 26, 19, 12, 1, 40, 51, 30, 36, 46, 54, 29, 39, 50, 44, 32, 47, 43, 48, 38, 55, 33, 52, 45, 41, 49, 35, 28, 31}
	var c, d uint32
	for i := 0; i < 28; i++ {
		c += qrcBitNum(key, keyPermC[i], 31-i)
		d += qrcBitNum(key, keyPermD[i], 31-i)
	}
	for i := 0; i < 16; i++ {
		shift := keyRndShift[i]
		c = ((c << shift) | (c >> (28 - shift))) & 0xfffffff0
		d = ((d << shift) | (d >> (28 - shift))) & 0xfffffff0
		toGen := i
		if mode == qrcDecrypt {
			toGen = 15 - i
		}
		for j := 0; j < 24; j++ {
			schedule[toGen][j/8] |= byte(qrcBitNumIntr(c, keyCompression[j], 7-(j%8)))
		}
		for j := 24; j < 48; j++ {
			schedule[toGen][j/8] |= byte(qrcBitNumIntr(d, keyCompression[j]-27, 7-(j%8)))
		}
	}
	return schedule
}
