package spec

// Levenshtein returns the edit distance between two strings.
func Levenshtein(a, b string) int {
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

// Suggest returns the closest match from valid within edit distance 2, or "".
func Suggest(input string, valid []string) string {
	best := ""
	bestDist := 3

	for _, v := range valid {
		d := Levenshtein(input, v)
		if d < bestDist {
			bestDist = d
			best = v
		}
	}

	return best
}
