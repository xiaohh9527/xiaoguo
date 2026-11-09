package hongguo

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type hongguoCommentNonce [11]byte

func newHongguoCommentNonce() (hongguoCommentNonce, error) {
	var nonce hongguoCommentNonce
	_, err := rand.Read(nonce[:])
	return nonce, err
}

func signHongguoCommentRequest(request *http.Request, nonce hongguoCommentNonce, now time.Time) {
	stamp := uint32(now.Unix())
	query := request.URL.RawQuery
	head2, head3 := (nonce[0]%7+1)<<5, 0xe0|nonce[1]&15
	seed := make([]byte, 20)
	hash := md5.Sum([]byte(query))
	copy(seed, hash[:4])
	copy(seed[12:16], []byte{0, 1, 7, 4})
	binary.BigEndian.PutUint32(seed[16:], stamp)
	key := [8]byte{0x4a, 0, 0x16, head3, 0x47, 0x6c, 0, head2}
	var box [256]byte
	for i := range box {
		box[i] = byte(i)
	}
	j := byte(0)
	for i := range box {
		j += box[i] + key[i&7]
		box[i] = box[j]
	}
	j = 0
	for offset := range seed {
		i := byte(offset + 1)
		j += box[i]
		box[i] = box[j]
		seed[offset] ^= box[box[i]+box[j]]
	}
	for i := range seed {
		seed[i] = bits.Reverse8(bits.RotateLeft8(seed[i], 4)^seed[(i+1)%len(seed)]) ^ 0xeb
	}
	request.Header.Set("X-Gorgon", hex.EncodeToString(append([]byte{0x84, 4, head2, head3, 0, 0}, seed...)))
	request.Header.Set("X-Khronos", strconv.FormatUint(uint64(stamp), 10))
	request.Header.Set("X-SS-Req-Ticket", strconv.FormatInt(now.UnixMilli(), 10))
	request.Header.Set("X-SS-STUB", "")
	request.Header.Set("X-Argus", hongguoCommentArgus(query, stamp, nonce))
	request.Header.Set("X-Ladon", hongguoCommentLadon(request.URL.Query().Get("aid"), stamp, nonce[2:6]))
}

func hongguoCommentPad(data []byte) []byte {
	padding := 16 - len(data)%16
	out := append([]byte(nil), data...)
	for i := 0; i < padding; i++ {
		out = append(out, byte(padding))
	}
	return out
}

func hongguoCommentLadon(aid string, timestamp uint32, prefix []byte) string {
	hash := md5.Sum(append(append([]byte(nil), prefix...), aid...))
	digest := []byte(hex.EncodeToString(hash[:]))
	left, right := binary.LittleEndian.Uint64(digest), binary.LittleEndian.Uint64(digest[8:])
	queue := []uint64{binary.LittleEndian.Uint64(digest[16:]), binary.LittleEndian.Uint64(digest[24:])}
	var keys [34]uint64
	for i := range keys {
		keys[i] = left
		mixed := (bits.RotateLeft64(right, -8) + left) ^ uint64(i)
		queue = append(queue, mixed)
		left = mixed ^ bits.RotateLeft64(left, 3)
		right, queue = queue[0], queue[1:]
	}
	data := hongguoCommentPad([]byte(strconv.FormatUint(uint64(timestamp), 10) + "-" + appLadonID + "-3019"))
	for offset := 0; offset < len(data); offset += 16 {
		left, right := binary.LittleEndian.Uint64(data[offset:]), binary.LittleEndian.Uint64(data[offset+8:])
		for _, key := range keys {
			right = key ^ (left + bits.RotateLeft64(right, -8))
			left = right ^ bits.RotateLeft64(left, 3)
		}
		binary.LittleEndian.PutUint64(data[offset:], left)
		binary.LittleEndian.PutUint64(data[offset+8:], right)
	}
	return base64.StdEncoding.EncodeToString(append(append([]byte(nil), prefix...), data...))
}

func hongguoCommentArgus(query string, timestamp uint32, nonce hongguoCommentNonce) string {
	key, _ := hex.DecodeString("ac1adaae95a7af94a5114ab3b3a97dd80050aa0a39314c40528caec95256c28c")
	var message []byte
	varint := func(field, value uint64) {
		message = binary.AppendUvarint(message, field*8)
		message = binary.AppendUvarint(message, value)
	}
	bytesField := func(field uint64, value []byte) {
		message = binary.AppendUvarint(message, field*8+2)
		message = binary.AppendUvarint(message, uint64(len(value)))
		message = append(message, value...)
	}
	varint(1, 0x20200929*2)
	varint(2, 2)
	varint(3, uint64(binary.LittleEndian.Uint32(nonce[6:10])))
	bytesField(4, []byte("3019"))
	bytesField(6, []byte(appLadonID))
	bytesField(7, []byte("6.8.1.32"))
	bytesField(8, []byte("v04.07.01-ml-android"))
	varint(9, 135135744)
	bytesField(10, make([]byte, 8))
	varint(12, uint64(timestamp)*2)
	bodyHash, queryHash := hongguoSM3(make([]byte, 16)), hongguoSM3([]byte(query))
	bytesField(13, bodyHash[:6])
	bytesField(14, queryHash[:6])
	nested := []byte{8, 2 + (nonce[10]%49)*2}
	for _, field := range []uint64{2, 3, 4} {
		nested = binary.AppendUvarint(nested, field*8)
		nested = binary.AppendUvarint(nested, 1388734)
	}
	bytesField(15, nested)
	bytesField(20, []byte("none"))
	varint(21, 738)
	data := hongguoCommentPad(message)
	seed := append(append(append([]byte(nil), key...), 0x3c, 0xcc, 0x56, 0x7b), key...)
	derived := hongguoSM3(seed)
	var rounds [72]uint64
	for i := 0; i < 4; i++ {
		rounds[i] = binary.LittleEndian.Uint64(derived[i*8:])
	}
	for i := 4; i < len(rounds); i++ {
		mixed := bits.RotateLeft64(rounds[i-1], -3) ^ rounds[i-3]
		mixed ^= bits.RotateLeft64(mixed, -1)
		rounds[i] = ^rounds[i-4] ^ mixed ^ (uint64(0x3dc94c3a046d678b)>>uint((i-4)%62))&1 ^ 3
	}
	for offset := 0; offset < len(data); offset += 16 {
		left, right := binary.LittleEndian.Uint64(data[offset:]), binary.LittleEndian.Uint64(data[offset+8:])
		for _, round := range rounds {
			left, right = right, left^(bits.RotateLeft64(right, 1)&bits.RotateLeft64(right, 8))^bits.RotateLeft64(right, 2)^round
		}
		binary.LittleEndian.PutUint64(data[offset:], left)
		binary.LittleEndian.PutUint64(data[offset+8:], right)
	}
	xor := []byte{0xd0, 0x4f, 0xfd, 0xff, 0xd0, 0x4f, 0xfd, 0xff}
	for i := range data {
		data[i] ^= xor[i%8]
	}
	data = append(xor, data...)
	for left, right := 0, len(data)-1; left < right; left, right = left+1, right-1 {
		data[left], data[right] = data[right], data[left]
	}
	container := append([]byte{0xa6, 0xe7, 0x83, 0xee, 0x70, 0x01, 0x10, 0x09, 0x18}, data...)
	container = hongguoCommentPad(append(container, 0x56, 0x7b))
	aesKey, iv := md5.Sum(key[:16]), md5.Sum(key[16:])
	block, _ := aes.NewCipher(aesKey[:])
	cipher.NewCBCEncrypter(block, iv[:]).CryptBlocks(container, container)
	return base64.StdEncoding.EncodeToString(append([]byte{0x3c, 0xcc}, container...))
}

func hongguoSM3(input []byte) [32]byte {
	length := uint64(len(input)) * 8
	data := append(append([]byte(nil), input...), 0x80)
	for len(data)%64 != 56 {
		data = append(data, 0)
	}
	data = binary.BigEndian.AppendUint64(data, length)
	state := [8]uint32{0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600, 0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e}
	for offset := 0; offset < len(data); offset += 64 {
		var words [68]uint32
		for i := 0; i < 16; i++ {
			words[i] = binary.BigEndian.Uint32(data[offset+i*4:])
		}
		for i := 16; i < len(words); i++ {
			v := words[i-16] ^ words[i-9] ^ bits.RotateLeft32(words[i-3], 15)
			words[i] = v ^ bits.RotateLeft32(v, 15) ^ bits.RotateLeft32(v, 23) ^ bits.RotateLeft32(words[i-13], 7) ^ words[i-6]
		}
		a, b, c, d, e, f, g, h := state[0], state[1], state[2], state[3], state[4], state[5], state[6], state[7]
		for i := 0; i < 64; i++ {
			t, ff, gg := uint32(0x79cc4519), a^b^c, e^f^g
			if i >= 16 {
				t, ff, gg = 0x7a879d8a, (a&b)|(a&c)|(b&c), (e&f)|(^e&g)
			}
			ss1 := bits.RotateLeft32(bits.RotateLeft32(a, 12)+e+bits.RotateLeft32(t, i), 7)
			tt1 := ff + d + (ss1 ^ bits.RotateLeft32(a, 12)) + (words[i] ^ words[i+4])
			tt2 := gg + h + ss1 + words[i]
			a, b, c, d = tt1, a, bits.RotateLeft32(b, 9), c
			e, f, g, h = tt2^bits.RotateLeft32(tt2, 9)^bits.RotateLeft32(tt2, 17), e, bits.RotateLeft32(f, 19), g
		}
		for i, value := range [8]uint32{a, b, c, d, e, f, g, h} {
			state[i] ^= value
		}
	}
	var result [32]byte
	for i, value := range state {
		binary.BigEndian.PutUint32(result[i*4:], value)
	}
	return result
}

func hongguoMixHash(seed uint32, data []byte) uint32 {
	h := seed
	for idx, b := range data {
		if idx&1 == 0 {
			h = (h << 6) ^ ((h >> 4) ^ h) ^ uint32(b)
		} else {
			h = ^(((h << 12) | uint32(b)) ^ ((h >> 7) ^ h))
		}
	}
	return h
}

func hongguoGateOK(query string, body []byte, stamp uint32) bool {
	qHash := sha256.Sum256([]byte(query))
	var bHash [16]byte
	if len(body) > 0 {
		bHash = md5.Sum(body)
	}
	var stampBytes [4]byte
	binary.LittleEndian.PutUint32(stampBytes[:], stamp)
	h := hongguoMixHash(539167016, qHash[:])
	h = hongguoMixHash(h, bHash[:])
	h = hongguoMixHash(h, stampBytes[:])
	return (h&15)%3 == 0
}

func hongguoSpeckHeader(aid, appID string, stamp uint32, prefix []byte) string {
	hash := md5.Sum(append(append([]byte(nil), prefix...), aid...))
	digest := []byte(hex.EncodeToString(hash[:]))
	left, right := binary.LittleEndian.Uint64(digest), binary.LittleEndian.Uint64(digest[8:])
	queue := []uint64{binary.LittleEndian.Uint64(digest[16:]), binary.LittleEndian.Uint64(digest[24:])}
	var keys [34]uint64
	for i := range keys {
		keys[i] = left
		mixed := (bits.RotateLeft64(right, -8) + left) ^ uint64(i)
		queue = append(queue, mixed)
		left = mixed ^ bits.RotateLeft64(left, 3)
		right, queue = queue[0], queue[1:]
	}
	data := hongguoCommentPad([]byte(fmt.Sprintf("%d-%s-%s", stamp, appID, aid)))
	for offset := 0; offset < len(data); offset += 16 {
		l, r := binary.LittleEndian.Uint64(data[offset:]), binary.LittleEndian.Uint64(data[offset+8:])
		for _, k := range keys {
			r = k ^ (l + bits.RotateLeft64(r, -8))
			l = r ^ bits.RotateLeft64(l, 3)
		}
		binary.LittleEndian.PutUint64(data[offset:], l)
		binary.LittleEndian.PutUint64(data[offset+8:], r)
	}
	return base64.StdEncoding.EncodeToString(append(append([]byte(nil), prefix...), data...))
}

func hongguoAppArgus(query string, body []byte, stamp uint32, aid, deviceID, versionName, deviceType, osVersion, channel string, nonce [11]byte) string {
	key, _ := hex.DecodeString("ac1adaae95a7af94a5114ab3b3a97dd80050aa0a39314c40528caec95256c28c")
	var message []byte
	varint := func(field, value uint64) {
		message = binary.AppendUvarint(message, field*8)
		message = binary.AppendUvarint(message, value)
	}
	bytesField := func(field uint64, value []byte) {
		message = binary.AppendUvarint(message, field*8+2)
		message = binary.AppendUvarint(message, uint64(len(value)))
		message = append(message, value...)
	}

	var nested15 []byte
	for _, fv := range [][2]uint64{{1, 1}, {2, 1}, {3, 1}, {7, 3348294860}} {
		nested15 = binary.AppendUvarint(nested15, fv[0]*8)
		nested15 = binary.AppendUvarint(nested15, fv[1])
	}

	var nested23 []byte
	appendBytes23 := func(field uint64, val string) {
		nested23 = binary.AppendUvarint(nested23, field*8+2)
		nested23 = binary.AppendUvarint(nested23, uint64(len(val)))
		nested23 = append(nested23, val...)
	}
	appendBytes23(1, deviceType)
	appendBytes23(2, osVersion)
	appendBytes23(3, channel)
	nested23 = binary.AppendUvarint(nested23, 4*8)
	nested23 = binary.AppendUvarint(nested23, 2147483648)

	varint(1, 0x20200929*2)
	varint(2, 2)
	varint(3, uint64(binary.LittleEndian.Uint32(nonce[6:10])&0x7fffffff))
	bytesField(4, []byte(aid))
	bytesField(5, []byte(deviceID))
	bytesField(6, []byte(appLadonID))
	bytesField(7, []byte(versionName))
	bytesField(8, []byte("v04.07.01-ml-android"))
	varint(9, 135135744)
	bytesField(10, make([]byte, 8))
	varint(11, 0)
	varint(12, uint64(stamp)*2)

	var bodyMD5 [16]byte
	if len(body) > 0 {
		bodyMD5 = md5.Sum(body)
	}
	bodyHash := hongguoSM3(bodyMD5[:])
	queryHash := hongguoSM3([]byte(query))
	bytesField(13, bodyHash[:6])
	bytesField(14, queryHash[:6])
	bytesField(15, nested15)
	bytesField(16, []byte{0})
	bytesField(20, []byte("none"))
	varint(21, 738)
	bytesField(23, nested23)
	varint(25, 2)

	data := hongguoCommentPad(message)
	seed := append(append(append([]byte(nil), key...), 0x3c, 0xcc, 0x56, 0x7b), key...)
	derived := hongguoSM3(seed)
	var rounds [72]uint64
	for i := 0; i < 4; i++ {
		rounds[i] = binary.LittleEndian.Uint64(derived[i*8:])
	}
	for i := 4; i < len(rounds); i++ {
		mixed := bits.RotateLeft64(rounds[i-1], -3) ^ rounds[i-3]
		mixed ^= bits.RotateLeft64(mixed, -1)
		rounds[i] = ^rounds[i-4] ^ mixed ^ (uint64(0x3dc94c3a046d678b)>>uint((i-4)%62))&1 ^ 3
	}
	for offset := 0; offset < len(data); offset += 16 {
		left, right := binary.LittleEndian.Uint64(data[offset:]), binary.LittleEndian.Uint64(data[offset+8:])
		for _, round := range rounds {
			left, right = right, left^(bits.RotateLeft64(right, 1)&bits.RotateLeft64(right, 8))^bits.RotateLeft64(right, 2)^round
		}
		binary.LittleEndian.PutUint64(data[offset:], left)
		binary.LittleEndian.PutUint64(data[offset+8:], right)
	}
	xor := []byte{0xd0, 0x4f, 0xfd, 0xff, 0xd0, 0x4f, 0xfd, 0xff}
	for i := range data {
		data[i] ^= xor[i%8]
	}
	data = append(xor, data...)
	for left, right := 0, len(data)-1; left < right; left, right = left+1, right-1 {
		data[left], data[right] = data[right], data[left]
	}
	container := append([]byte{0xa6, 0xe7, 0x83, 0xee, 0x70, 0x01, 0x10, 0x09, 0x18}, data...)
	container = hongguoCommentPad(append(container, 0x56, 0x7b))
	aesKey, iv := md5.Sum(key[:16]), md5.Sum(key[16:])
	block, _ := aes.NewCipher(aesKey[:])
	cipher.NewCBCEncrypter(block, iv[:]).CryptBlocks(container, container)
	return base64.StdEncoding.EncodeToString(append([]byte{0x3c, 0xcc}, container...))
}

func signHongguoAppPlayRequest(req *http.Request, body []byte, now time.Time) {
	nowMs := now.UnixMilli()
	stamp := uint32(nowMs / 1000)

	q := req.URL.Query()
	aid := firstNonEmpty(q.Get("aid"), appAid)
	deviceID := q.Get("device_id")
	versionName := firstNonEmpty(q.Get("version_name"), appVersionName)
	deviceType := firstNonEmpty(q.Get("device_type"), appDeviceType)
	osVersion := firstNonEmpty(q.Get("os_version"), appOSVersion)
	channel := firstNonEmpty(q.Get("channel"), appChannelDefault)

	baseQuery := req.URL.RawQuery
	var finalQuery string
	var rticket int64
	for offset := int64(0); offset < 128; offset++ {
		rticket = nowMs + offset
		candidate := fmt.Sprintf("%s&ts=%d&_rticket=%d", baseQuery, stamp, rticket)
		if hongguoGateOK(candidate, body, stamp) {
			finalQuery = candidate
			break
		}
	}
	if finalQuery == "" {
		rticket = nowMs
		finalQuery = fmt.Sprintf("%s&ts=%d&_rticket=%d", baseQuery, stamp, rticket)
	}
	req.URL.RawQuery = finalQuery

	var nonce [15]byte
	_, _ = rand.Read(nonce[:])

	md5q := md5.Sum([]byte(finalQuery))
	var md5b [16]byte
	if len(body) > 0 {
		md5b = md5.Sum(body)
	}
	seed := make([]byte, 20)
	copy(seed[0:4], md5q[:4])
	copy(seed[4:8], md5b[:4])
	copy(seed[12:16], []byte{0, 4, 6, 4})
	binary.BigEndian.PutUint32(seed[16:20], stamp)

	head0, head1 := nonce[0], nonce[1]
	key := [8]byte{74, 64, 22, head1, 71, 108, 1, head0}
	var box [256]byte
	for i := range box {
		box[i] = byte(i)
	}
	j := byte(0)
	for i := 0; i < 256; i++ {
		j += box[i] + key[i&7]
		box[i], box[j] = box[j], box[i]
	}
	iIdx, jIdx := byte(0), byte(0)
	for k := range seed {
		iIdx++
		jIdx += box[iIdx]
		box[iIdx], box[jIdx] = box[jIdx], box[iIdx]
		seed[k] ^= box[byte(box[jIdx]*2)]
	}
	for k := 0; k < len(seed); k += 2 {
		next := seed[0]
		if k+1 < len(seed) {
			next = seed[k+1]
		}
		x := next ^ bits.RotateLeft8(seed[k], 4)
		seed[k] = ^(bits.Reverse8(x) ^ 0x14)
	}

	var argusNonce [11]byte
	copy(argusNonce[:], nonce[:11])

	var traceRand [24]byte
	_, _ = rand.Read(traceRand[:])
	traceID := fmt.Sprintf("00-%x-%x-01", traceRand[:16], traceRand[16:24])
	reqRand := uint64(binary.LittleEndian.Uint32(nonce[11:15])) % 2000000000

	req.Header.Set("x-ss-req-ticket", strconv.FormatInt(rticket, 10))
	req.Header.Set("sdk-version", "2")
	req.Header.Set("passport-sdk-version", appPassportSDKVer)
	req.Header.Set("x-xs-from-web", "0")
	req.Header.Set("x-khronos", strconv.FormatUint(uint64(stamp), 10))
	req.Header.Set("x-ladon", hongguoSpeckHeader(aid, appLadonID, stamp, nonce[2:6]))
	req.Header.Set("x-argus", hongguoAppArgus(finalQuery, body, stamp, aid, deviceID, versionName, deviceType, osVersion, channel, argusNonce))
	req.Header.Set("x-gorgon", hex.EncodeToString(append([]byte{0x84, 0x04, head0, head1, 0x40, 0x01}, seed...)))
	req.Header.Set("x-helios", hongguoSpeckHeader(aid, appHeliosID, stamp, nonce[11:15]))
	req.Header.Set("x-vc-bdturing-sdk-version", appBdturingSDKVer)
	req.Header.Set("lc", appLcID)
	req.Header.Set("x-ss-dp", aid)
	req.Header.Set("x-reading-request", fmt.Sprintf("%d-%d", rticket, reqRand))
	req.Header.Set("x-ss-stub", strings.ToUpper(hex.EncodeToString(md5b[:])))
	req.Header.Set("x-tt-trace-id", traceID)
}

func hongguoNewUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func hongguoTTEncrypt(data []byte) ([]byte, error) {
	kdfSalt, _ := hex.DecodeString("4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58")
	var rnd [32]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return nil, err
	}
	d1 := sha512.Sum512(rnd[:])
	d2Input := append(append([]byte(nil), d1[:]...), kdfSalt...)
	d2 := sha512.Sum512(d2Input)
	key, iv := d2[:16], d2[16:32]
	salt := sha512.Sum512(data)
	plain := hongguoCommentPad(append(append([]byte(nil), salt[:]...), data...))

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ct := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, plain)

	out := make([]byte, 0, 6+32+len(ct))
	out = append(out, 0x74, 0x63, 0x05, 0x10, 0x00, 0x00)
	out = append(out, rnd[:]...)
	out = append(out, ct...)
	return out, nil
}

func (c *Client) RegisterNewDevice(ctx context.Context) (DeviceProfile, error) {
	nowMs := time.Now().UnixMilli()
	cdid := hongguoNewUUID()
	clientUDID := hongguoNewUUID()
	reqID := hongguoNewUUID()
	var openUDIDBytes [8]byte
	_, _ = rand.Read(openUDIDBytes[:])
	openUDID := hex.EncodeToString(openUDIDBytes[:])

	regPayload := map[string]any{
		"magic_tag": "ss_app_log",
		"header": map[string]any{
			"display_name":            "红果免费短剧",
			"update_version_code":     73932,
			"manifest_version_code":   73932,
			"app_version_minor":       appVersionName,
			"aid":                     8662,
			"channel":                 appChannelRegistered,
			"package":                 appPackage,
			"app_version":             appVersionName,
			"version_code":            73932,
			"sdk_version":             appSDKVersion,
			"sdk_target_version":      appSDKTgtVersion,
			"os":                      "Android",
			"os_version":              appOSVersion,
			"os_api":                  32,
			"device_model":            appDeviceType,
			"device_brand":            appDeviceBrand,
			"device_manufacturer":     appDeviceManu,
			"cpu_abi":                 "arm64-v8a",
			"density_dpi":             520,
			"display_density":         "mdpi",
			"resolution":              "2772*1280",
			"language":                appLanguage,
			"timezone":                8,
			"access":                  "wifi",
			"not_request_sender":      0,
			"carrier":                 "CHINA MOBILE",
			"mcc_mnc":                 "46000",
			"rom":                     "1135",
			"rom_version":             appRomVersion,
			"git_hash":                appGitHash,
			"sig_hash":                appSigHash,
			"release_build":           appReleaseBuild,
			"cdid":                    cdid,
			"openudid":                openUDID,
			"clientudid":              clientUDID,
			"region":                  "CN",
			"tz_name":                 "Asia/Shanghai",
			"tz_offset":               28800,
			"sim_region":              "cn",
			"sim_serial_number":       []any{},
			"oaid_may_support":        false,
			"req_id":                  reqID,
			"device_platform":         appDevicePlatform,
			"custom": map[string]any{
				"account_region_src": "fallback",
				"host_bit":           64,
				"account_region":     "cn",
				"dragon_device_type": "phone",
			},
			"pre_installed_channel":  "",
			"apk_first_install_time": nowMs - 86400000,
			"is_system_app":          0,
			"sdk_flavor":             "china",
			"guest_mode":             0,
		},
		"_gen_time": nowMs,
	}

	rawJSON, err := json.Marshal(regPayload)
	if err != nil {
		return DeviceProfile{}, err
	}

	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	_, _ = gz.Write(rawJSON)
	_ = gz.Close()

	encBody, err := hongguoTTEncrypt(gzBuf.Bytes())
	if err != nil {
		return DeviceProfile{}, err
	}

	md5Sum := md5.Sum(encBody)
	stub := strings.ToUpper(hex.EncodeToString(md5Sum[:]))
	ua := appUserAgent

	var traceRand [24]byte
	_, _ = rand.Read(traceRand[:])
	traceID := fmt.Sprintf("00-%x-%x-01", traceRand[:16], traceRand[16:24])

	regHosts := []string{
		"https://log5-applog-lq.fqnovel.com",
		"https://log5-applog-hl.fqnovel.com",
		"https://log5-applog-lf.fqnovel.com",
	}

	var lastErr error
	for _, host := range regHosts {
		regURL := host + pathDeviceRegister
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, regURL, bytes.NewReader(encBody))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("log-encode-type", "gzip")
		req.Header.Set("x-ss-req-ticket", strconv.FormatInt(nowMs, 10))
		req.Header.Set("x-vc-bdturing-sdk-version", "4.0.3.cn")
		req.Header.Set("Content-Type", "application/octet-stream;tt-data=a")
		req.Header.Set("x-ss-stub", stub)
		req.Header.Set("x-ss-dp", "8662")
		req.Header.Set("x-tt-trace-id", traceID)
		req.Header.Set("User-Agent", ua)
		req.Header.Set("x-neptune", "-11|50:51:59")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK || len(respBody) == 0 {
			lastErr = fmt.Errorf("register HTTP %d", resp.StatusCode)
			continue
		}

		var res map[string]any
		dec := json.NewDecoder(bytes.NewReader(respBody))
		dec.UseNumber()
		if err := dec.Decode(&res); err != nil {
			lastErr = err
			continue
		}

		devID := mapString(res, "device_id_str", "device_id")
		iid := mapString(res, "install_id_str", "install_id", "iid")
		if devID == "" || devID == "0" || iid == "" || iid == "0" {
			lastErr = errors.New("device_register returned empty device_id")
			continue
		}

		var ttreq string
		for _, ck := range resp.Cookies() {
			if ck.Name == "ttreq" && ck.Value != "" {
				ttreq = ck.Value
			}
		}

		profile := DeviceProfile{
			DeviceID:    devID,
			InstallID:   iid,
			CDID:        cdid,
			OpenUDID:    openUDID,
			KlinkEgdi:   mapString(res, "klink_egdi"),
			DeviceToken: mapString(res, "device_token"),
			TTReq:       ttreq,
			DeviceBrand: appDeviceBrand,
			DeviceType:  appDeviceType,
			OSVersion:   appOSVersion,
			OSAPI:       appOSAPI,
			Resolution:  appResolution,
			DPI:         appDPI,
			Channel:     appChannelRegistered,
			UserAgent:   ua,
		}

		c.mu.Lock()
		c.deviceProfiles = append([]DeviceProfile{profile}, c.deviceProfiles...)
		c.deviceIndex = 0
		c.mu.Unlock()

		if data, err := json.MarshalIndent(profile, "", "  "); err == nil {
			_ = os.MkdirAll("data", 0o755)
			_ = os.WriteFile("data/device.json", data, 0o644)
		}

		return profile, nil
	}

	if lastErr == nil {
		lastErr = errors.New("device registration failed")
	}
	return DeviceProfile{}, lastErr
}
