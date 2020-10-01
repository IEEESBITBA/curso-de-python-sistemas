package actions

import (
	"math"
	"math/big"
)

// These are
const (
	Padding   = '='
	AbcASCII  = "\u0000\u0001\u0002\u0003\u0004\u0005\u0006\a\b\t\n\v\f\n\u000E\u000F\u0010\u0011\u0012\u0013\u0014\u0015\u0016\u0017\u0018\u0019\u001A\u001B\u001C\u001D\u001E\u001F !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~\u007F"
	Abc64     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	Abc64safe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	Abc10     = "0123456789"
	AbcABC    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	Abcabc    = "abcdefghijklmnopqrstuvwxyz"
	AbcUID    = Abcabc + Abc10 + "-"
)

// Encode encodes rune message to given alphabet
func Encode(msg []rune, alphabet string) string {
	length := int(math.Log2(float64(len(alphabet))))
	shift := make([]bool, 0, length)
	bCounter := 0
	var R []rune
	for _, b := range msg {
		for j := 7; j >= 0; j-- {
			bCounter++
			shift = append(shift, b&(1<<j) > 0)
			if bCounter%length == 0 {
				var nextRune rune
				for bit := range shift {
					nextRune <<= 1
					if shift[bit] {
						nextRune++
					}
				}
				R = append(R, rune(alphabet[nextRune]))
				shift = make([]bool, 0, length)
			}
		}
	}
	// the messa
	if len(shift) > 0 {
		for len(shift) < length {
			shift = append(shift, false)
		}
		var nextRune rune
		for bit := range shift {
			nextRune <<= 1
			if shift[bit] {
				nextRune++
			}
		}
		R = append(R, rune(alphabet[nextRune]))
	}
	return string(R)
}

// ToBase changes base of a number to len(alphabet).
func ToBase(N *big.Int, alphabet string) string {
	var num big.Int
	num.Set(N)
	amap := make(map[int64]rune)
	for i, v := range alphabet {
		amap[int64(i)] = v
	}
	var R []rune
	zero := big.NewInt(0)
	base := big.NewInt(int64(len(alphabet)))
	for {
		var mod big.Int
		mod.Mod(&num, base)
		R = append([]rune{amap[mod.Int64()]}, R...)
		num.Div(&num, base)
		if num.Cmp(zero) == 0 {
			break
		}
	}
	return string(R)
}

// ToNum changes representation of a number in base len(alphabet)
// to it's math/big Int representation.
func ToNum(R []rune, alphabet string) *big.Int {
	amap := make(map[rune]*big.Int)
	for i, v := range alphabet {
		amap[v] = big.NewInt(int64(i))
	}
	var sum, base, exp, aux, idx big.Int
	base = *big.NewInt(int64(len(alphabet)))
	msgLength := big.NewInt(int64(len(R)))
	for i, char := range R {
		val, prs := amap[char]
		if !prs {
			panic("char not in alphabet")
		}
		idx.SetInt64(int64(i + 1))
		aux.Sub(msgLength, &idx)
		exp.Exp(&base, &aux, nil)
		sum.Add(&sum, val.Mul(val, &exp))
	}
	return &sum
}
