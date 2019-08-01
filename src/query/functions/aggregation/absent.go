// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package aggregation

import (
	"fmt"
	"math"
	"time"

	"github.com/m3db/m3/src/query/block"
	"github.com/m3db/m3/src/query/executor/transform"
	"github.com/m3db/m3/src/query/functions/utils"
	"github.com/m3db/m3/src/query/models"
	"github.com/m3db/m3/src/query/parser"
)

const (
	// AbsentType returns 1 if there are no elements in this step, or if no series
	// are present in the current block.
	AbsentType = "absent"
)

// NewAbsentOp creates a new absent operation.
func NewAbsentOp() parser.Params {
	return newAbsentOp()
}

// absentOp stores required properties for absent ops.
type absentOp struct{}

// OpType for the operator.
func (o absentOp) OpType() string {
	return AbsentType
}

// String representation.
func (o absentOp) String() string {
	return fmt.Sprintf("type: absent")
}

// Node creates an execution node.
func (o absentOp) Node(
	controller *transform.Controller,
	_ transform.Options,
) transform.OpNode {
	return &absentNode{
		op:         o,
		controller: controller,
	}
}

func newAbsentOp() absentOp {
	return absentOp{}
}

// absentNode is different from base node as it uses no grouping and has
// special handling for the 0-series case.
type absentNode struct {
	op         parser.Params
	controller *transform.Controller
}

func (n *absentNode) Params() parser.Params {
	return n.op
}

// Process the block
func (n *absentNode) Process(queryCtx *models.QueryContext,
	ID parser.NodeID, b block.Block) error {
	return transform.ProcessSimpleBlock(n, n.controller, queryCtx, ID, b)
}

func (n *absentNode) ProcessBlock(queryCtx *models.QueryContext,
	ID parser.NodeID, b block.Block) (block.Block, error) {
	stepIter, err := b.StepIter()
	if err != nil {
		return nil, err
	}

	// Absent should
	var (
		meta        = stepIter.Meta()
		seriesMetas = stepIter.SeriesMeta()
		tagOpts     = meta.Tags.Opts
	)

	// If no series in the input, return a scalar block with value 1.
	if len(seriesMetas) == 0 {
		return block.NewScalar(
			func(_ time.Time) float64 { return 1 },
			meta.Bounds,
			tagOpts,
		), nil
	}

	// NB: pull any common tags out into the created series.
	dupeTags, _ := utils.DedupeMetadata(seriesMetas, tagOpts)
	mergedCommonTags := meta.Tags.Add(dupeTags)
	meta.Tags = models.NewTags(0, tagOpts)
	emptySeriesMeta := []block.SeriesMeta{
		block.SeriesMeta{
			Tags: mergedCommonTags,
			Name: []byte{},
		},
	}

	builder, err := n.controller.BlockBuilder(queryCtx, meta, emptySeriesMeta)
	if err != nil {
		return nil, err
	}

	if err = builder.AddCols(1); err != nil {
		return nil, err
	}

	for index := 0; stepIter.Next(); index++ {
		step := stepIter.Current()
		values := step.Values()

		var val float64 = 1
		for _, v := range values {
			if !math.IsNaN(v) {
				val = 0
				break
			}
		}

		if err := builder.AppendValue(index, val); err != nil {
			return nil, err
		}
	}

	if err = stepIter.Err(); err != nil {
		return nil, err
	}

	return builder.Build(), nil
}
