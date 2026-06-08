package gojxl

// General Modular channel decode: per-pixel property computation, MA-tree
// traversal, and all predictors (libjxl modular/encoding/encoding.cc +
// context_predict.h). Replaces the single-leaf WP-only path with the full
// algorithm so multi-node trees, every predictor, and inter-channel reference
// properties are supported.

const (
	kNumNonrefProperties  = kNumStaticProperties + 13 + 1 // = 16 (weighted::kNumProperties = 1)
	kWPProp               = kNumNonrefProperties - 1      // = 15
	kExtraPropsPerChannel = 4
)

// modChannel is one Modular image channel.
type modChannel struct {
	w, h           int
	hshift, vshift int
	pix            []int32
}

func (c *modChannel) at(x, y int) int64 { return int64(c.pix[y*c.w+x]) }

func clampedGradient(n, w, l int64) int64 {
	m := minI64(n, w)
	M := maxI64(n, w)
	grad := n + w - l
	if l < m {
		grad = M
	}
	if l > M {
		return m
	}
	return grad
}

func selectPred(a, b, c int64) int64 {
	p := a + b - c
	if absI64(p-a) < absI64(p-b) {
		return a
	}
	return b
}

func predictOne(pred uint32, left, top, toptop, topleft, topright, leftleft, toprightright, wpPred int64) int64 {
	switch pred {
	case predZero:
		return 0
	case predLeft:
		return left
	case predTop:
		return top
	case predSelect:
		return selectPred(left, top, topleft)
	case predWeighted:
		return wpPred
	case predGradient:
		return clampedGradient(left, top, topleft)
	case predTopLeft:
		return topleft
	case predTopRight:
		return topright
	case predLeftLeft:
		return leftleft
	case predAverage0:
		return divTrunc(left+top, 2)
	case predAverage1:
		return divTrunc(left+topleft, 2)
	case predAverage2:
		return divTrunc(topleft+top, 2)
	case predAverage3:
		return divTrunc(top+topright, 2)
	case predAverage4:
		return divTrunc(6*top-2*toptop+7*left+leftleft+toprightright+3*topright+8, 16)
	}
	return 0
}

// divTrunc truncates toward zero (matching C++ integer division).
func divTrunc(a, b int64) int64 { return a / b }

func computeNumProps(tree []treeNode) int {
	maxProp := -1
	for _, n := range tree {
		if n.property > maxProp {
			maxProp = n.property
		}
	}
	if maxProp+1 > kNumNonrefProperties {
		return divCeilInt(maxProp+1-kNumNonrefProperties, kExtraPropsPerChannel)*kExtraPropsPerChannel + kNumNonrefProperties
	}
	return kNumNonrefProperties
}

func divCeilInt(a, b int) int { return (a + b - 1) / b }

// traverseTree walks the MA tree to a leaf, returning the leaf node.
// lchild = the property>splitval branch; rchild = property<=splitval (matching
// ValidateTree bounds in dec_ma.cc).
func traverseTree(tree []treeNode, props []int64) *treeNode {
	pos := 0
	for tree[pos].property != -1 {
		n := &tree[pos]
		if props[n.property] > int64(n.splitval) {
			pos = n.lchild
		} else {
			pos = n.rchild
		}
	}
	return &tree[pos]
}

// referenceChannels returns the indices of prior channels usable as reference
// properties for channel ci (same dims/shifts), in libjxl order (ci-1 down).
func referenceChannels(image []modChannel, ci, maxRefs int) []int {
	var refs []int
	cur := &image[ci]
	for j := ci - 1; j >= 0 && len(refs) < maxRefs; j-- {
		c := &image[j]
		if c.w != cur.w || c.h != cur.h || c.hshift != cur.hshift || c.vshift != cur.vshift {
			continue
		}
		refs = append(refs, j)
	}
	return refs
}

// decodeChannel decodes channel ci of image using the (global or local) tree.
func decodeChannel(reader *ansSymbolReader, b *bitReader, tree []treeNode,
	ctxMap []uint8, image []modChannel, ci, groupID int, wpH wpHeader) {
	ch := &image[ci]
	w, h := ch.w, ch.h
	if w == 0 || h == 0 {
		return
	}
	numProps := computeNumProps(tree)
	numRefs := (numProps - kNumNonrefProperties) / kExtraPropsPerChannel
	refChans := referenceChannels(image, ci, numRefs)

	props := make([]int64, numProps)
	props[0] = int64(ci)
	props[1] = int64(groupID)
	wp := newWPState(wpH, w)

	// Per-row reference property buffer: refStride entries per pixel.
	refStride := len(refChans) * kExtraPropsPerChannel
	refs := make([]int64, w*refStride)

	get := func(x, y int) int64 { return int64(ch.pix[y*w+x]) }

	for y := 0; y < h; y++ {
		props[2] = int64(y)
		props[9] = 0
		// Precompute reference properties for this row.
		if refStride > 0 {
			for i := range refs {
				refs[i] = 0
			}
			off := 0
			for _, j := range refChans {
				rc := &image[j]
				for x := 0; x < w; x++ {
					v := rc.at(x, y)
					var vleft int64
					if x > 0 {
						vleft = rc.at(x-1, y)
					}
					vtop := vleft
					if y > 0 {
						vtop = rc.at(x, y-1)
					}
					vtopleft := vleft
					if x > 0 && y > 0 {
						vtopleft = rc.at(x-1, y-1)
					}
					vpred := clampedGradient(vleft, vtop, vtopleft)
					base := x*refStride + off
					refs[base+0] = absI64(v)
					refs[base+1] = v
					refs[base+2] = absI64(v - vpred)
					refs[base+3] = v - vpred
				}
				off += kExtraPropsPerChannel
			}
		}

		for x := 0; x < w; x++ {
			var left, top, topleft, topright, leftleft, toptop, toprightright int64
			if x > 0 {
				left = get(x-1, y)
			} else if y > 0 {
				left = get(x, y-1)
			}
			top = left
			if y > 0 {
				top = get(x, y-1)
			}
			topleft = left
			if x > 0 && y > 0 {
				topleft = get(x-1, y-1)
			}
			topright = top
			if x+1 < w && y > 0 {
				topright = get(x+1, y-1)
			}
			leftleft = left
			if x > 1 {
				leftleft = get(x-2, y)
			}
			toptop = top
			if y > 1 {
				toptop = get(x, y-2)
			}
			toprightright = topright
			if x+2 < w && y > 0 {
				toprightright = get(x+2, y-1)
			}

			props[3] = int64(x)
			props[4] = absI64(top)
			props[5] = absI64(left)
			props[6] = top
			props[7] = left
			props[8] = left - props[9] // uses previous pixel's props[9]
			props[9] = left + top - topleft
			props[10] = left - topleft
			props[11] = topleft - top
			props[12] = top - topright
			props[13] = top - toptop
			props[14] = left - leftleft

			wpPred := wp.predictProp(x, y, w, top, left, topright, topleft, toptop, props, kWPProp)

			for i := 0; i < refStride; i++ {
				props[kNumNonrefProperties+i] = refs[x*refStride+i]
			}

			leaf := traverseTree(tree, props)
			ctx := int(ctxMap[leaf.lchild])
			guess := predictOne(leaf.predictor, left, top, toptop, topleft, topright, leftleft, toprightright, wpPred)
			v := reader.readHybridUintClustered(ctx, b)
			val := int64(unpackSigned32(v))*int64(leaf.multiplier) + leaf.predOffset + guess
			ch.pix[y*w+x] = int32(val)
			wp.updateErrors(val, x, y, w)
		}
	}
}
