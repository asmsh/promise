// Package uniquerand is a light version of github.com/asmsh/uniquerand
package uniquerand

import (
	"math/rand"
)

// defRandSrc is the random generator used by default.
// it's a function takes an integer, r, and returns a random number in range [0, r).
var defRandSrc = rand.Intn

// defRange is the default range used for the zero value of the Int.
const defRange = 10

const blockSize = 32

type blockType = uint32

// Int allows returning unique random numbers within a predefined range.
// It depends on another source for randomness, keeps track of all generated
// numbers, and makes sure that the returned number is unique.
// The zero value produces unique numbers using math/rand in range [0, 10).
type Int struct {
	r  int         // range
	m  blockType   // block num 0
	em []blockType // block num 1+
}

// Reset sets the range of the Int generator and resets all previous memory.
// If the given range is less than or equal to zero, the default range (10) is used.
func (uri *Int) Reset(r int) {
	if r <= 0 {
		r = defRange
	}

	// reset the default fields
	uri.r = r
	uri.m = 0
	uri.em = nil

	// check if we need the extra memory
	l := r / blockSize
	if int(r%blockSize) == 0 {
		l = l - 1
	}
	if l != 0 {
		uri.em = make([]blockType, l)
	}
}

// Range returns the current range of the Int generator, which is the exclusive
// upper limit of the unique random number that could be generated, starting from 0.
func (uri *Int) Range() int {
	if uri.r > 0 {
		return uri.r
	}
	return defRange
}

func (uri *Int) has(n int) (bn int, mb, tm, mm blockType) {
	// get the Block Number
	bn = n / blockSize

	// get the respective Memory Block
	mb = uri.m
	if bn > 0 {
		mb = uri.em[bn-1]
	}

	sv := n % blockSize     // Shift Value
	tm = blockType(1 << sv) // Target Mask
	mm = mb & tm            // Masked Memory
	return
}

// Get returns a unique random number and ok as true.
// If ok is false, it means that we ran out of unique numbers within the specified range.
func (uri *Int) Get() (urn int, ok bool) {
	grn := defRandSrc(uri.Range()) // Generated Random Number

	// Block Number, Memory Block, Target Mask, Masked Memory
	bn, mb, tm, mm := uri.has(grn)

	// Generated Random Number was not generated before
	if mm == 0 {
		// update the respective Memory Block
		if bn > 0 {
			uri.em[bn-1] = mb | tm
		} else {
			uri.m = mb | tm
		}
		urn = grn // Unique Random Number
		return urn, true
	}

	// Generated Random Number was generated before
	return uri.getSlow()
}

func (uri *Int) getSlow() (urn int, ok bool) {
	// loop over the default memory to find the first block that has a zero bit
	for j := 0; j < blockSize; j++ {
		tm := blockType(1 << j) // current block's Target Mask
		mm := uri.m & tm        // current block's Masked Memory
		if mm != 0 {
			continue // the current bit is not zero
		}
		uri.m = uri.m | tm // update the respective Memory Block
		urn = j            // calculate the Unique Random Number
		if urn < uri.Range() {
			return urn, true
		}
		return 0, false
	}

	// loop over the extra memory to find the first block that has a zero bit
	for i, m := range uri.em {
		// if this block is all 0s, simply set it to 1 and return
		if m == 0 {
			uri.em[i] = 1       // update the respective Memory Block
			urn = i * blockSize // calculate the Unique Random Number
			urn += blockSize    // add the base default memory size
			return urn, true
		}

		// otherwise, search for the first 0 in this block
		for j := 0; j < blockSize; j++ {
			tm := blockType(1 << j) // current block's Target Mask
			mm := m & tm            // current block's Masked Memory
			if mm != 0 {
				continue // the current bit is not zero
			}
			uri.em[i] = m | tm    // update the respective Memory Block
			urn = i*blockSize + j // calculate the Unique Random Number
			urn += blockSize      // add the base default memory size
			if urn < uri.Range() {
				return urn, true
			}
			return 0, false
		}
	}

	return 0, false
}

func (uri *Int) Put(num int) (ok bool) {
	if num < 0 || num >= uri.Range() {
		return false
	}

	// Block Number, Memory Block, Target Mask, Masked Memory
	bn, mb, tm, mm := uri.has(num)

	// num is already available (not consumed by Get)
	if mm == 0 {
		return false
	}

	// update the respective Memory Block
	if bn > 0 {
		uri.em[bn-1] = mb &^ tm
	} else {
		uri.m = mb &^ tm
	}

	return true
}
