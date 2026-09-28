package hongguo

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
)

// DecodeHongguoContentKey parses spade_a into an AES-128 key.
func DecodeHongguoContentKey(value string) ([]byte, error) {
	if len(value) > 1024 {
		return nil, errors.New("红果媒体密钥数据过长")
	}
	value = strings.TrimSpace(value)
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(value)
		if err != nil || len(raw) < 3 {
			return nil, errors.New("红果媒体密钥编码无效")
		}
	}
	tagLength := int(raw[0]^raw[1]^raw[2]) - 48
	contentLength := len(raw) - tagLength - 1
	if tagLength < 1 || contentLength < 33 || contentLength >= len(raw) {
		return nil, errors.New("红果媒体密钥结构无效")
	}
	seed := raw[len(raw)-tagLength-2] ^ raw[len(raw)-tagLength-1]
	tag := make([]byte, tagLength)
	for index := range tag {
		tag[index] = raw[len(raw)-tagLength+index] ^ seed
	}
	if string(tag) == "app_v2" || string(tag) == "web_v2" {
		return nil, errors.New("红果媒体密钥版本暂不支持")
	}
	decoded := make([]byte, contentLength)
	previousEven, previousOdd := byte(250), byte(85)
	for index, current := range raw[1 : 1+contentLength] {
		previous := previousEven
		if index%2 == 0 {
			previousEven = current
		} else {
			previous = previousOdd
			previousOdd = current
		}
		decoded[index] = byte(int(previous^current) - 21 - bits.OnesCount(uint(index)))
	}
	padding, err := strconv.ParseUint(string(decoded[:1]), 36, 8)
	if err != nil || contentLength-int(padding)-1 != 32 {
		return nil, errors.New("红果媒体密钥内容无效")
	}
	key, err := hex.DecodeString(string(decoded[1:33]))
	if err != nil || len(key) != aes.BlockSize {
		return nil, fmt.Errorf("红果媒体密钥不是有效的 AES-128 密钥")
	}
	return key, nil
}

// DecryptCENCMP4 decrypts an ISO-BMFF MP4 file encrypted with CENC (AES-128-CTR) in place.
// Returns the decrypted standard MP4 byte slice.
func DecryptCENCMP4(data []byte, key []byte) ([]byte, error) {
	if len(key) != aes.BlockSize {
		return nil, errors.New("AES key must be 16 bytes")
	}

	// Locate moov box
	var moovOffset, moovSize int
	for offset := 0; offset+8 <= len(data); {
		sz := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		name := string(data[offset+4 : offset+8])
		if sz == 0 {
			sz = len(data) - offset
		}
		if sz < 8 || offset+sz > len(data) {
			break
		}
		if name == "moov" {
			moovOffset = offset
			moovSize = sz
			break
		}
		offset += sz
	}
	if moovSize == 0 {
		return nil, errors.New("moov box not found")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Make a copy so original is intact
	out := append([]byte(nil), data...)

	curMoov := out[moovOffset+8 : moovOffset+moovSize]
	for tOffset := 0; tOffset+8 <= len(curMoov); {
		tSz := int(binary.BigEndian.Uint32(curMoov[tOffset : tOffset+4]))
		tName := string(curMoov[tOffset+4 : tOffset+8])
		if tSz < 8 || tOffset+tSz > len(curMoov) {
			break
		}
		if tName == "trak" {
			trakData := curMoov[tOffset+8 : tOffset+tSz]
			_ = decryptTrack(trakData, out, block)
		}
		tOffset += tSz
	}

	return out, nil
}

func decryptTrack(trakData []byte, fullFile []byte, block cipher.Block) error {
	stbl := findSubBox(trakData, "mdia", "minf", "stbl")
	if stbl == nil {
		return nil
	}

	stsd := findDirectBox(stbl, "stsd")
	if stsd == nil || len(stsd) < 16 {
		return nil
	}

	entryOffset := 16
	if entryOffset+8 > len(stsd) {
		return nil
	}
	entrySize := int(binary.BigEndian.Uint32(stsd[entryOffset : entryOffset+4]))
	entryFmt := string(stsd[entryOffset+4 : entryOffset+8])
	if entryFmt != "encv" && entryFmt != "enca" {
		return nil // Not encrypted
	}

	entryData := stsd[entryOffset+8 : entryOffset+entrySize]
	var sinf []byte
	for sOffset := 0; sOffset+8 <= len(entryData); sOffset++ {
		if string(entryData[sOffset+4:sOffset+8]) == "sinf" {
			sz := int(binary.BigEndian.Uint32(entryData[sOffset : sOffset+4]))
			if sz >= 8 && sOffset+sz <= len(entryData) {
				sinf = entryData[sOffset : sOffset+sz]
				break
			}
		}
	}
	if sinf == nil {
		return fmt.Errorf("sinf not found in %s", entryFmt)
	}

	var frma []byte
	for fOffset := 0; fOffset+8 <= len(sinf); fOffset++ {
		if string(sinf[fOffset+4:fOffset+8]) == "frma" {
			sz := int(binary.BigEndian.Uint32(sinf[fOffset : fOffset+4]))
			if sz >= 12 && fOffset+sz <= len(sinf) {
				frma = sinf[fOffset : fOffset+sz]
				break
			}
		}
	}
	if frma == nil || len(frma) < 12 {
		return fmt.Errorf("frma not found")
	}
	origFmt := frma[8:12] // e.g. "hvc1" or "avc1" or "mp4a"

	senc := findDirectBox(stbl, "senc")
	if senc == nil || len(senc) < 16 {
		return fmt.Errorf("senc not found")
	}
	sencFlags := binary.BigEndian.Uint32([]byte{0, senc[9], senc[10], senc[11]})
	sampleCount := binary.BigEndian.Uint32(senc[12:16])
	hasSubsamples := (sencFlags & 0x000002) != 0

	stsz := findDirectBox(stbl, "stsz")
	if stsz == nil || len(stsz) < 20 {
		return fmt.Errorf("stsz not found")
	}
	defaultSampleSize := binary.BigEndian.Uint32(stsz[12:16])
	stszCount := binary.BigEndian.Uint32(stsz[16:20])
	sampleSizes := make([]uint32, stszCount)
	if defaultSampleSize != 0 {
		for i := range sampleSizes {
			sampleSizes[i] = defaultSampleSize
		}
	} else {
		for i := 0; i < int(stszCount); i++ {
			sampleSizes[i] = binary.BigEndian.Uint32(stsz[20+i*4 : 24+i*4])
		}
	}

	stsc := findDirectBox(stbl, "stsc")
	if stsc == nil || len(stsc) < 16 {
		return fmt.Errorf("stsc not found")
	}
	stscCount := binary.BigEndian.Uint32(stsc[12:16])
	type stscEntry struct {
		firstChunk      uint32
		samplesPerChunk uint32
	}
	stscEntries := make([]stscEntry, stscCount)
	for i := 0; i < int(stscCount); i++ {
		stscEntries[i] = stscEntry{
			firstChunk:      binary.BigEndian.Uint32(stsc[16+i*12 : 20+i*12]),
			samplesPerChunk: binary.BigEndian.Uint32(stsc[20+i*12 : 24+i*12]),
		}
	}

	var chunkOffsets []int64
	if stco := findDirectBox(stbl, "stco"); stco != nil && len(stco) >= 16 {
		count := binary.BigEndian.Uint32(stco[12:16])
		chunkOffsets = make([]int64, count)
		for i := 0; i < int(count); i++ {
			chunkOffsets[i] = int64(binary.BigEndian.Uint32(stco[16+i*4 : 20+i*4]))
		}
	} else if co64 := findDirectBox(stbl, "co64"); co64 != nil && len(co64) >= 16 {
		count := binary.BigEndian.Uint32(co64[12:16])
		chunkOffsets = make([]int64, count)
		for i := 0; i < int(count); i++ {
			chunkOffsets[i] = int64(binary.BigEndian.Uint64(co64[16+i*8 : 24+i*8]))
		}
	} else {
		return fmt.Errorf("stco/co64 not found")
	}

	sampleOffsets := make([]int64, len(sampleSizes))
	sampleIdx := 0
	for chunkIdx := 0; chunkIdx < len(chunkOffsets); chunkIdx++ {
		chunkNum := uint32(chunkIdx + 1)
		var samplesInChunk uint32
		for s := len(stscEntries) - 1; s >= 0; s-- {
			if chunkNum >= stscEntries[s].firstChunk {
				samplesInChunk = stscEntries[s].samplesPerChunk
				break
			}
		}
		curOffset := chunkOffsets[chunkIdx]
		for s := uint32(0); s < samplesInChunk && sampleIdx < len(sampleSizes); s++ {
			sampleOffsets[sampleIdx] = curOffset
			curOffset += int64(sampleSizes[sampleIdx])
			sampleIdx++
		}
	}

	sencDataOffset := 16
	for i := 0; i < int(sampleCount) && i < len(sampleOffsets); i++ {
		if sencDataOffset+8 > len(senc) {
			break
		}
		ivBytes := senc[sencDataOffset : sencDataOffset+8]
		var iv [16]byte
		copy(iv[:8], ivBytes)
		sencDataOffset += 8

		var subsamples []struct{ clear, protected uint32 }
		if hasSubsamples {
			if sencDataOffset+2 > len(senc) {
				break
			}
			subCount := binary.BigEndian.Uint16(senc[sencDataOffset : sencDataOffset+2])
			sencDataOffset += 2
			for sub := 0; sub < int(subCount); sub++ {
				if sencDataOffset+6 > len(senc) {
					break
				}
				c := uint32(binary.BigEndian.Uint16(senc[sencDataOffset : sencDataOffset+2]))
				p := binary.BigEndian.Uint32(senc[sencDataOffset+2 : sencDataOffset+6])
				sencDataOffset += 6
				subsamples = append(subsamples, struct{ clear, protected uint32 }{c, p})
			}
		}

		sOffset := sampleOffsets[i]
		sSize := int64(sampleSizes[i])
		if sOffset+sSize > int64(len(fullFile)) {
			continue
		}

		stream := cipher.NewCTR(block, iv[:])
		if !hasSubsamples {
			stream.XORKeyStream(fullFile[sOffset:sOffset+sSize], fullFile[sOffset:sOffset+sSize])
		} else {
			pos := sOffset
			for _, sub := range subsamples {
				pos += int64(sub.clear)
				if pos+int64(sub.protected) <= sOffset+sSize {
					stream.XORKeyStream(fullFile[pos:pos+int64(sub.protected)], fullFile[pos:pos+int64(sub.protected)])
					pos += int64(sub.protected)
				}
			}
		}
	}

	// Change encv/enca to original format (e.g. hvc1, avc1, mp4a)
	copy(stsd[entryOffset+4:entryOffset+8], origFmt)
	// Neutralize sinf: rename to 'free' so player skips DRM parsing
	copy(sinf[4:8], []byte("free"))

	return nil
}

func findDirectBox(data []byte, name string) []byte {
	offset := 0
	for offset+8 <= len(data) {
		sz := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		bname := string(data[offset+4 : offset+8])
		if sz == 0 {
			sz = len(data) - offset
		}
		if sz < 8 || offset+sz > len(data) {
			break
		}
		if bname == name {
			return data[offset : offset+sz]
		}
		offset += sz
	}
	return nil
}

func findSubBox(data []byte, path ...string) []byte {
	cur := data
	for _, target := range path {
		found := false
		offset := 0
		for offset+8 <= len(cur) {
			sz := int(binary.BigEndian.Uint32(cur[offset : offset+4]))
			bname := string(cur[offset+4 : offset+8])
			if sz == 0 {
				sz = len(cur) - offset
			}
			if sz < 8 || offset+sz > len(cur) {
				break
			}
			if bname == target {
				cur = cur[offset+8 : offset+sz]
				found = true
				break
			}
			offset += sz
		}
		if !found {
			return nil
		}
	}
	return cur
}
