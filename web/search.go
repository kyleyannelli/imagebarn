/*
* The code for the below Sørensen–Dice coefficient is optimized from https://medium.com/@fernandoporazzi/strings-similarity-in-golang-ee7e4814d796
*
* Below tests were ran to determine suitability:
* Note: Every test generated all random strings before timing tests
*
* 1. Random 1000000 Strings @ Length Of 50 Characters Each
*   - Unoptimized: Average of 4.383 seconds over 5 runs
*   - Optimized: Average of 456 milliseconds over 5 runs
*
* 2. Random 1000000 Strings @ Length of 20 Characters Each
*   - Unoptimized: Average of 1.650 seconds over 5 runs
*   - Optimized: Average of 297 milliseconds over 5 runs
*
* 3. Random 1000000 Strings @ Length of 10 Characters Each
*   - Unoptimized: Average of 636 milliseconds over 5 runs
*   - Optimized: Average of 243 milliseconds over 5 runs
*
* 4. Random 1000000 Strings @ Length of 5 Characters Each
*   - Unoptimized: Average of 186 milliseconds over 5 runs
*   - Optimized: Average of 227 milliseconds over 5 runs

* 5. Random 1,000 Strings @ 50 Characters Each
*   - Unoptimized: Average of 7.2451 ms over 5 runs
*   - Optimized: Average of 1.0654 ms over 5 runs
*
* 6. Random 1,000 Strings @ 30 Characters Each
*   - Unoptimized: Average of 3.2389 ms over 5 runs
*   - Optimized: Average of 0.7381 ms over 5 runs
*
* 7. Random 1,000 Strings @ 20 Characters Each
*   - Unoptimized: Average of 1.7767 ms over 5 runs
*   - Optimized: Average of 0.3466 ms over 5 runs
*
* 8. Random 1,000 Strings @ 10 Characters Each
*   - Unoptimized: Average of 1.2038 ms over 5 runs
*   - Optimized: Average of 0.5244 ms over 5 runs
*
* 9. Random 1,000 Strings @ 5 Characters Each
*   - Unoptimized: Average of 0.2972 ms over 5 runs
*   - Optimized: Average of 0.5159 ms over 5 runs
*
* 10. 100 Random Strings @ Compare with 30 Characters and 5 Characters
*   - Unoptimized: Average of 0.4419 ms over 5 runs
*   - Optimized: Average of 0.1319 ms over 5 runs
*
* 11. 100 Random Strings @ Compare with 30 Characters and 1 Character
*   - Unoptimized: Average of 0.0250 ms over 5 runs
*   - Optimized: Average of 0.0464 ms over 5 runs
*
* 13. 100 Random Strings @ Compare with 30 Characters and 20 Characters
*   - Unoptimized: Average of 0.4686 ms over 5 runs
*   - Optimized: Average of 0.1386 ms over 5 runs
 */
package web

import (
	"strings"
	"sync"
)

var (
	countsPool = sync.Pool{
		New: func() interface{} {
			return make([]int, 65536)
		},
	}
	indicesPool = sync.Pool{
		New: func() interface{} {
			return make([]int, 0, 100)
		},
	}
)

func returnEarlyIfPossible(stringOne, stringTwo string) float32 {
	if len(stringOne) == 0 && len(stringTwo) == 0 {
		return 1.0
	}
	if len(stringOne) == 0 || len(stringTwo) == 0 {
		return 0.0
	}
	if stringOne == stringTwo {
		return 1.0
	}
	if len(stringOne) < 2 || len(stringTwo) < 2 {
		return 0.0
	}
	return -1.0
}

func CompareTwoStringsOptimized(stringOne, stringTwo string) float32 {
	counts1 := countsPool.Get().([]int)
	counts2 := countsPool.Get().([]int)
	defer countsPool.Put(counts1)
	defer countsPool.Put(counts2)

	indices1 := indicesPool.Get().([]int)
	indices2 := indicesPool.Get().([]int)
	defer indicesPool.Put(indices1[:0])
	defer indicesPool.Put(indices2[:0])

	indices1 = indices1[:0]
	indices2 = indices2[:0]

	stringOne = strings.ReplaceAll(stringOne, " ", "")
	stringTwo = strings.ReplaceAll(stringTwo, " ", "")

	if value := returnEarlyIfPossible(stringOne, stringTwo); value >= 0 {
		return value
	}

	for i := 0; i < len(stringOne)-1; i++ {
		index := (int(stringOne[i]) << 8) | int(stringOne[i+1])
		if counts1[index] == 0 {
			indices1 = append(indices1, index)
		}
		counts1[index]++
	}

	for i := 0; i < len(stringTwo)-1; i++ {
		index := (int(stringTwo[i]) << 8) | int(stringTwo[i+1])
		if counts2[index] == 0 {
			indices2 = append(indices2, index)
		}
		counts2[index]++
	}

	var intersectionSize float32
	for _, index := range indices1 {
		if counts2[index] > 0 {
			intersectionSize += float32(min(counts1[index], counts2[index]))
		}
	}

	totalBigrams := float32(len(stringOne) + len(stringTwo) - 2)
	result := (2.0 * intersectionSize) / totalBigrams

	for _, index := range indices1 {
		counts1[index] = 0
	}
	for _, index := range indices2 {
		counts2[index] = 0
	}

	return result
}
