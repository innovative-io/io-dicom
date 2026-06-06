package gojxl

// Inverse Modular transforms (libjxl modular/transform/*). Currently: RCT
// (reversible color transform). Palette and Squeeze are added as needed.

// invRCT applies the inverse reversible color transform in place over channels
// [beginC, beginC+2] (rct.cc InvRCT + InvRCTRow).
func invRCT(image []modChannel, beginC int, rctType uint32) {
	if rctType == 0 {
		return
	}
	permutation := int(rctType / 7)
	custom := int(rctType % 7)
	m := beginC
	w := image[m].w
	h := image[m].h
	c0 := image[m].pix
	c1 := image[m+1].pix
	c2 := image[m+2].pix
	// Output channel index mapping for the permutation.
	o0 := m + permutation%3
	o1 := m + (permutation+1+permutation/3)%3
	o2 := m + (permutation+2-permutation/3)%3
	out0 := image[o0].pix
	out1 := image[o1].pix
	out2 := image[o2].pix

	second := custom >> 1
	third := custom & 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if custom == 6 { // YCoCg
				Y := c0[i]
				Co := c1[i]
				Cg := c2[i]
				tmp := Y - (Cg >> 1)
				G := Cg + tmp
				B := tmp - (Co >> 1)
				R := B + Co
				out0[i] = R
				out1[i] = G
				out2[i] = B
				continue
			}
			first := c0[i]
			sec := c1[i]
			thr := c2[i]
			if third != 0 {
				thr = thr + first
			}
			if second == 1 {
				sec = sec + first
			} else if second == 2 {
				sec = sec + ((first + thr) >> 1)
			}
			out0[i] = first
			out1[i] = sec
			out2[i] = thr
		}
	}
}

// applyInverseTransforms undoes the GroupHeader transforms in reverse order.
// Only RCT is supported so far; Palette/Squeeze return false (unsupported).
func applyInverseTransforms(image []modChannel, transforms []transform, nbMeta int) bool {
	for i := len(transforms) - 1; i >= 0; i-- {
		t := transforms[i]
		switch t.id {
		case transformRCT:
			invRCT(image, nbMeta+int(t.beginC), t.rctType)
		default:
			return false // Palette/Squeeze not yet implemented
		}
	}
	return true
}
