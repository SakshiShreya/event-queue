package queue

// In this package, every ticket owns a permanent slot (its arrival number) and
// the tree stores 1 for slots still waiting, 0 for everyone else. A waiting
// ticket's position is then just the prefix sum up to it's own slot.
type Fenwick struct {
	tree []int // tree[i] covers the range (i-lowbit(i), i), index 0 is unused
	raw  []int // raw[i] is the plain value at slot i, kept so grow can rebuild
	n    int   // number of usable slots
}

// NewFenwick returns a tree sized for capacity slots. It grows on demand, so
// the capacity is only a starting hint.
func NewFenwick(capacity int) *Fenwick {
	if capacity < 1 {
		capacity = 1
	}

	return &Fenwick{
		tree: make([]int, capacity+1),
		raw:  make([]int, capacity+1),
		n:    capacity,
	}
}

// lowbit isolates the lowest set bit, which is the width of the range a node covers.
func lowbit(i int) int { return i & -i }

// Add applies delta to slot i, growing the tree if i is past the end.
func (f *Fenwick) Add(i, delta int) {
	if i < 1 || delta == 0 {
		return
	}
	if i > f.n {
		f.grow(i)
	}

	f.raw[i] += delta

	for ; i <= f.n; i += lowbit(i) {
		f.tree[i] += delta
	}
}

// PrefixSum returns the sum of slots 1...i
func (f *Fenwick) PrefixSum(i int) int {
	if i > f.n {
		i = f.n
	}

	sum := 0
	for ; i > 0; i -= lowbit(i) {
		sum += f.tree[i]
	}
	return sum
}

// RangeSum returns the sum of l...r inclusive
func (f *Fenwick) RangeSum(l, r int) int {
	if l > r {
		return 0
	}

	return f.PrefixSum(r) - f.PrefixSum(l-1)
}

// Total returns total sum of every slot
func (f *Fenwick) Total() int { return f.PrefixSum(f.n) }

// FindKth returns the smallest slot whose prefix sum reaches k, i.e. the slot
// holding the k-th set element. It returns 0 when fewer than k elements are set.
//
// This is a binary-lifting descent: walk from the highest power of two
// downwards, stepping into a subtree only while its sum is still short of k.
func (f *Fenwick) FindKth(k int) int {
	if k < 1 || k > f.Total() {
		return 0
	}

	pos := 0
	for step := f.highestPowerOfTwo(); step > 0; step >>= 1 {
		if next := pos + step; next <= f.n && f.tree[next] < k {
			pos = next
			k -= f.tree[next]
		}
	}

	return pos + 1
}

func (f *Fenwick) highestPowerOfTwo() int {
	p := 1
	for p<<1 <= f.n {
		p <<= 1
	}
	return p
}

// grow doubles capacity until size fits, then rebuilds the tree from raw values.
// A rebuild is O(n), but capacity doubles, so it costs O(1) amortised per Add.
func (f *Fenwick) grow(size int) {
	newN := f.n
	for newN < size {
		newN *= 2
	}

	raw := make([]int, newN+1)
	copy(raw, f.raw)

	tree := make([]int, newN+1)
	copy(tree, raw)

	for i := 1; i <= newN; i++ {
		if parent := i + lowbit(i); parent <= newN {
			tree[parent] += tree[i]
		}
	}

	f.raw, f.tree, f.n = raw, tree, newN
}
