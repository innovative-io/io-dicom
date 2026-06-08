package gojxl

// Self-correcting weighted predictor (libjxl modular/encoding/context_predict.h
// namespace weighted::State).

const (
	kNumWPPredictors = 4
	kPredExtraBits   = 3
	kPredictionRound = ((1 << kPredExtraBits) >> 1) - 1 // = 3
)

var wpDivLookup = [64]uint32{
	16777216, 8388608, 5592405, 4194304, 3355443, 2796202, 2396745, 2097152,
	1864135, 1677721, 1525201, 1398101, 1290555, 1198372, 1118481, 1048576,
	986895, 932067, 883011, 838860, 798915, 762600, 729444, 699050,
	671088, 645277, 621378, 599186, 578524, 559240, 541200, 524288,
	508400, 493447, 479349, 466033, 453438, 441505, 430185, 419430,
	409200, 399457, 390167, 381300, 372827, 364722, 356962, 349525,
	342392, 335544, 328965, 322638, 316551, 310689, 305040, 299593,
	294337, 289262, 284359, 279620, 275036, 270600, 266305, 262144,
}

func floorLog2U64(x uint64) int {
	n := -1
	for x != 0 {
		n++
		x >>= 1
	}
	return n
}

type wpState struct {
	header     wpHeader
	xsize      int
	predErrors [kNumWPPredictors][]uint32 // (xsize+2)*2 each
	errors     []int64                    // (xsize+2)*2
	prediction [kNumWPPredictors]int64
	pred       int64
}

func newWPState(header wpHeader, xsize int) *wpState {
	s := &wpState{header: header, xsize: xsize}
	n := (xsize + 2) * 2
	for i := range s.predErrors {
		s.predErrors[i] = make([]uint32, n)
	}
	s.errors = make([]int64, n)
	return s
}

func wpAddBits(x int64) int64 { return x << kPredExtraBits }

func (s *wpState) errorWeight(x uint64, maxweight uint32) uint32 {
	shift := floorLog2U64(x+1) - 5
	if shift < 0 {
		shift = 0
	}
	return 4 + ((maxweight * wpDivLookup[x>>uint(shift)]) >> uint(shift))
}

func (s *wpState) weightedAverage(p [kNumWPPredictors]int64, w [kNumWPPredictors]uint32) int64 {
	var weightSum uint32
	for i := 0; i < kNumWPPredictors; i++ {
		weightSum += w[i]
	}
	logWeight := floorLog2U64(uint64(weightSum)) // >= 4
	weightSum = 0
	for i := 0; i < kNumWPPredictors; i++ {
		w[i] >>= uint(logWeight - 4)
		weightSum += w[i]
	}
	sum := int64(weightSum>>1) - 1
	for i := 0; i < kNumWPPredictors; i++ {
		sum += p[i] * int64(w[i])
	}
	return (sum * int64(wpDivLookup[weightSum-1])) >> 24
}

// predictProp is predict() but also writes the weighted-predictor property
// (max abs neighbor error) into props[propOff] — the compute_properties path
// of State::Predict.
func (s *wpState) predictProp(x, y, xsize int, N, W, NE, NW, NN int64, props []int64, propOff int) int64 {
	var curRow, prevRow int
	if y&1 != 0 {
		prevRow = xsize + 2
	} else {
		curRow = xsize + 2
	}
	posN := prevRow + x
	posNE := posN
	if x < xsize-1 {
		posNE = posN + 1
	}
	posNW := posN
	if x > 0 {
		posNW = posN - 1
	}
	var teW int64
	if x != 0 {
		teW = s.errors[curRow+x-1]
	}
	teN := s.errors[posN]
	teNW := s.errors[posNW]
	teNE := s.errors[posNE]
	p := teW
	if absI64(teN) > absI64(p) {
		p = teN
	}
	if absI64(teNW) > absI64(p) {
		p = teNW
	}
	if absI64(teNE) > absI64(p) {
		p = teNE
	}
	props[propOff] = p
	return s.predict(x, y, xsize, N, W, NE, NW, NN)
}

// predict returns the weighted-predictor guess for pixel (x,y) given neighbors.
func (s *wpState) predict(x, y, xsize int, N, W, NE, NW, NN int64) int64 {
	var curRow, prevRow int
	if y&1 != 0 {
		curRow = 0
		prevRow = xsize + 2
	} else {
		curRow = xsize + 2
		prevRow = 0
	}
	posN := prevRow + x
	posNE := posN
	if x < xsize-1 {
		posNE = posN + 1
	}
	posNW := posN
	if x > 0 {
		posNW = posN - 1
	}
	var weights [kNumWPPredictors]uint32
	for i := 0; i < kNumWPPredictors; i++ {
		w := uint64(s.predErrors[i][posN]) + uint64(s.predErrors[i][posNE]) + uint64(s.predErrors[i][posNW])
		weights[i] = s.errorWeight(w, s.header.w[i])
	}

	N = wpAddBits(N)
	W = wpAddBits(W)
	NE = wpAddBits(NE)
	NW = wpAddBits(NW)
	NN = wpAddBits(NN)

	var teW int64
	if x != 0 {
		teW = s.errors[curRow+x-1]
	}
	teN := s.errors[posN]
	teNW := s.errors[posNW]
	sumWN := teN + teW
	teNE := s.errors[posNE]

	s.prediction[0] = W + NE - N
	s.prediction[1] = N - (((sumWN + teNE) * int64(s.header.p1C)) >> 5)
	s.prediction[2] = W - (((sumWN + teNW) * int64(s.header.p2C)) >> 5)
	s.prediction[3] = N - ((teNW*int64(s.header.p3Ca) + teN*int64(s.header.p3Cb) +
		teNE*int64(s.header.p3Cc) + (NN-N)*int64(s.header.p3Cd) +
		(NW-W)*int64(s.header.p3Ce)) >> 5)

	s.pred = s.weightedAverage(s.prediction, weights)

	if ((teN ^ teW) | (teN ^ teNW)) > 0 {
		return (s.pred + kPredictionRound) >> kPredExtraBits
	}
	mx := maxI64(W, maxI64(NE, N))
	mn := minI64(W, minI64(NE, N))
	s.pred = maxI64(mn, minI64(mx, s.pred))
	return (s.pred + kPredictionRound) >> kPredExtraBits
}

func (s *wpState) updateErrors(val int64, x, y, xsize int) {
	var curRow, prevRow int
	if y&1 != 0 {
		curRow = 0
		prevRow = xsize + 2
	} else {
		curRow = xsize + 2
		prevRow = 0
	}
	val = wpAddBits(val)
	s.errors[curRow+x] = s.pred - val
	for i := 0; i < kNumWPPredictors; i++ {
		err := (absI64(s.prediction[i]-val) + kPredictionRound) >> kPredExtraBits
		s.predErrors[i][curRow+x] = uint32(err)
		s.predErrors[i][prevRow+x+1] += uint32(err)
	}
}

func maxI64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
func minI64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
func absI64(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}
