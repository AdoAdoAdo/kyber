package cipher

import (
	"hash"
	"crypto/cipher"
	"crypto/hmac"
	"github.com/dedis/crypto/ints"
)

type blockCipherState struct {

	// Configuration state
	newCipher func(key []byte) (cipher.Block, error)
	newHash func() hash.Hash
	blockLen, keyLen, hashLen int
	iv []byte	// initialization vector for counter mode

	// Per-message cipher state
	k []byte	// master secret state from last message, 0 if unkeyed
	h Hash		// hash or hmac for absorbing input
	s Stream	// stream cipher for encrypting, nil if none
}

const bufLen = 1024

var zeroBytes = make([]byte, bufLen)

// BlockCipherState creates a general symmetric cipher State
// built from a block cipher and a cryptographic hash function.
func BlockCipherState(newCipher func(key []byte) (cipher.Block, error),
			newHash func() hash.Hash,
			blockLen, keyLen, hashLen int) BlockState {
	bcs := blockCipherState{}
	bcs.newCipher = newCipher
	bcs.newHash = newHash
	bcs.blockLen = blockLen
	bcs.keyLen = keyLen
	bcs.hashLen = hashLen

	bcs.h = bcs.newHash()
	return &bcs
}

func (bcs *blockCipherState) crypt(dst, src []byte, more, enc bool) {

	for len(dst) > 0 {
		if len(src) == 0 {
			src = zeroBytes
		}
		l := ints.Min(len(dst), len(src))

		if bcs.s == nil {
			if bcs.k == nil {
				bcs.k = make([]byte, bcs.hashLen)
				bcs.iv = make([]byte, bcs.blockLen)
			}
			b, err := bcs.newCipher(bcs.k[:bcs.keyLen])
			if err != nil {
				panic(err.Error())
			}
			bcs.s = cipher.NewCTR(b, bcs.iv)
		}

		if enc {
			bcs.s.XORKeyStream(dst[:l], src[:l])
			bcs.h.Write(dst[:l])	// encrypt-then-MAC
		} else {
			bcs.h.Write(src[:l])	// MAC-then-decrypt
			bcs.s.XORKeyStream(dst[:l], src[:l])
		}

		src = src[l:]
		dst = dst[l:]
	}
	if len(src) > 0 {
		bcs.h.Write(src)	// absorb extra src bytes
	}
	if !more {
		bcs.k = bcs.h.Sum(bcs.k[:0]) // update state with absorbed data
		bcs.h = hmac.New(bcs.newHash, bcs.k)	// ready for next msg
		bcs.s = nil
	}
}

func (bcs *blockCipherState) BlockEncrypt(dst, src []byte, more bool) {
	bcs.crypt(dst, src, more, true)
}

func (bcs *blockCipherState) BlockDecrypt(dst, src []byte, more bool) {
	bcs.crypt(dst, src, more, false)
}

func (bcs *blockCipherState) Encrypt(dst, src []byte) {
	bcs.crypt(dst, src, false, true)
}

func (bcs *blockCipherState) Decrypt(dst, src []byte) {
	bcs.crypt(dst, src, false, false)
}

func (bcs *blockCipherState) KeyLen() int {
	return bcs.keyLen
}

func (bcs *blockCipherState) HashLen() int {
	return bcs.hashLen
}

func (bcs *blockCipherState) BlockSize() int {
	return 1	// incremental encrypt/decrypt work at any granularity
}

func (bcs *blockCipherState) Clone() State {
	if bcs.s != nil {
		panic("cannot clone cipher state mid-message")
	}

	nbcs := *bcs
	if bcs.k != nil {	// keyed state
		nbcs.k = make([]byte, bcs.hashLen)
		copy(nbcs.k, bcs.k)
		nbcs.h = hmac.New(nbcs.newHash, nbcs.k)
	} else {		// unkeyed state
		nbcs.h = nbcs.newHash()
	}
	return &nbcs
}

