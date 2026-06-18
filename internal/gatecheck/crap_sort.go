package gatecheck

import "sort"

func sortCrapOffenders(offenders []CrapOffender) {
	sort.SliceStable(offenders, func(leftIndex, rightIndex int) bool {
		left := offenders[leftIndex]
		right := offenders[rightIndex]
		if left.Crap != right.Crap {
			return left.Crap > right.Crap
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Name < right.Name
	})
}
