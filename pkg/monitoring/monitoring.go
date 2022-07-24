/*
 *  Copyright (c) 2022 Avesha, Inc. All rights reserved.
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package monitoring

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/kubeslice/worker-operator/pkg/logger"
	"go.opencensus.io/metric"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricproducer"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// Unit encodes the standard name for describing the quantity
// measured by a Metric (if applicable).
type Unit string

var (
	log = logger.NewLogger().WithValues("type", "monitoring")
)

// Predefined units for use with the monitoring package.
const (
	None         Unit = "1"
	Bytes        Unit = "By"
	Seconds      Unit = "s"
	Milliseconds Unit = "ms"
)

type (
	// A Metric collects numerical observations.
	Metric interface {
		// Increment records a value of 1 for the current measure. For Sums,
		// this is equivalent to adding 1 to the current value. For Gauges,
		// this is equivalent to setting the value to 1. For Distributions,
		// this is equivalent to making an observation of value 1.
		Increment()

		// Decrement records a value of -1 for the current measure. For Sums,
		// this is equivalent to subtracting -1 to the current value. For Gauges,
		// this is equivalent to setting the value to -1. For Distributions,
		// this is equivalent to making an observation of value -1.
		Decrement()

		// Name returns the name value of a Metric.
		Name() string

		// Record makes an observation of the provided value for the given measure.
		Record(value float64)

		// RecordInt makes an observation of the provided value for the measure.
		RecordInt(value int64)
		// With creates a new Metric, with the LabelValues provided. This allows creating
		// a set of pre-dimensioned data for recording purposes. This is primarily used
		// for documentation and convenience. Metrics created with this method do not need
		// to be registered (they share the registration of their parent Metric).
		With(labelValues ...LabelValue) Metric

		// Register configures the Metric for export. It MUST be called before collection
		// of values for the Metric. An error will be returned if registration fails.
		Register() error
	}

	// DerivedMetrics can be used to supply values that dynamically derive from internal
	// state, but are not updated based on any specific event. Their value will be calculated
	// based on a value func that executes when the metrics are exported.
	//
	// At the moment, only a Gauge type is supported.
	DerivedMetric interface {
		// Name returns the name value of a DerivedMetric.
		Name() string

		// Register handles any required setup to ensure metric export.
		Register() error
	}

	// Options encode changes to the options passed to a Metric at creation time.
	Options func(*options)

	// A Label provides a named dimension for a Metric.
	Label tag.Key

	// A LabelValue represents a Label with a specific value. It is used to record
	// values for a Metric.
	LabelValue tag.Mutator

	options struct {
		unit     Unit
		labels   []Label
		useInt64 bool
	}

	// RecordHook has a callback function which a measure is recorded.
	RecordHook interface {
		OnRecordFloat64Measure(f *stats.Float64Measure, tags []tag.Mutator, value float64)
		OnRecordInt64Measure(i *stats.Int64Measure, tags []tag.Mutator, value int64)
	}
)

var (
	recordHooks     map[string]RecordHook
	recordHookMutex sync.RWMutex

	derivedRegistry = metric.NewRegistry()
)

func init() {
	recordHooks = make(map[string]RecordHook)
	// ensures exporters can see any derived metrics
	metricproducer.GlobalManager().AddProducer(derivedRegistry)
}

// RegisterRecordHook adds a RecordHook for a given measure.
func RegisterRecordHook(name string, h RecordHook) {
	recordHookMutex.Lock()
	defer recordHookMutex.Unlock()
	recordHooks[name] = h
}

// WithLabels provides configuration options for a new Metric, providing the expected
// dimensions for data collection for that Metric.
func WithLabels(labels ...Label) Options {
	return func(opts *options) {
		opts.labels = labels
	}
}

// WithUnit provides configuration options for a new Metric, providing unit of measure
// information for a new Metric.
func WithUnit(unit Unit) Options {
	return func(opts *options) {
		opts.unit = unit
	}
}

// WithInt64Values provides configuration options for a new Metric, indicating that
// recorded values will be saved as int64 values. Any float64 values recorded will
// converted to int64s via math.Floor-based conversion.
func WithInt64Values() Options {
	return func(opts *options) {
		opts.useInt64 = true
	}
}

// Value creates a new LabelValue for the Label.
func (l Label) Value(value string) LabelValue {
	return tag.Upsert(tag.Key(l), value)
}

// MustCreateLabel will attempt to create a new Label. If
// creation fails, then this method will panic.
func MustCreateLabel(key string) Label {
	k, err := tag.NewKey(key)
	if err != nil {
		panic(fmt.Errorf("could not create label %q: %v", key, err))
	}
	return Label(k)
}

// MustRegister is a helper function that will ensure that the provided Metrics are
// registered. If a metric fails to register, this method will panic.
func MustRegister(metrics ...Metric) {
	for _, m := range metrics {
		if err := m.Register(); err != nil {
			panic(err)
		}
	}
}

// NewSum creates a new Metric with an aggregation type of Sum (the values will be cumulative).
// That means that data collected by the new Metric will be summed before export.
func NewSum(name, description string, opts ...Options) Metric {
	return newMetric(name, description, view.Sum(), opts...)
}

// NewGauge creates a new Metric with an aggregation type of LastValue. That means that data collected
// by the new Metric will export only the last recorded value.
func NewGauge(name, description string, opts ...Options) Metric {
	return newMetric(name, description, view.LastValue(), opts...)
}

// NewDerivedGauge creates a new Metric with an aggregation type of LastValue that generates the value
// dynamically according to the provided function. This can be used for values based on querying some
// state within a system (when event-driven recording is not appropriate).
// NOTE: Labels not currently supported.
func NewDerivedGauge(name, description string, valueFn func() float64) DerivedMetric {
	m, err := derivedRegistry.AddFloat64DerivedGauge(name, metric.WithDescription(description), metric.WithUnit(metricdata.UnitDimensionless))
	if err != nil {
		log.Error(err, "failed to add metric %q", name)
	}
	err = m.UpsertEntry(valueFn)
	if err != nil {
		log.Error(err, "failed to upsert entry for %q", name)
	}
	return &derivedFloat64Metric{m, name}
}

// NewDistribution creates a new Metric with an aggregation type of Distribution. This means that the
// data collected by the Metric will be collected and exported as a histogram, with the specified bounds.
func NewDistribution(name, description string, bounds []float64, opts ...Options) Metric {
	return newMetric(name, description, view.Distribution(bounds...), opts...)
}

func newMetric(name, description string, aggregation *view.Aggregation, opts ...Options) Metric {
	o := createOptions(opts...)
	if o.useInt64 {
		return newInt64Metric(name, description, aggregation, o)
	}
	return newFloat64Metric(name, description, aggregation, o)
}

type derivedFloat64Metric struct {
	*metric.Float64DerivedGauge

	name string
}

func (d *derivedFloat64Metric) Name() string {
	return d.name
}

// no-op
func (d *derivedFloat64Metric) Register() error {
	return nil
}

type float64Metric struct {
	*stats.Float64Measure

	// tags stores all tags for the metrics
	tags []tag.Mutator
	// ctx is a precomputed context holding tags, as an optimization
	ctx  context.Context
	view *view.View

	incrementMeasure []stats.Measurement
	decrementMeasure []stats.Measurement
}

func createOptions(opts ...Options) *options {
	o := &options{unit: None, labels: make([]Label, 0)}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func newFloat64Metric(name, description string, aggregation *view.Aggregation, opts *options) *float64Metric {
	measure := stats.Float64(name, description, string(opts.unit))
	tagKeys := make([]tag.Key, 0, len(opts.labels))
	for _, l := range opts.labels {
		tagKeys = append(tagKeys, tag.Key(l))
	}
	ctx, _ := tag.New(context.Background()) //nolint:errcheck
	return &float64Metric{
		Float64Measure:   measure,
		tags:             make([]tag.Mutator, 0),
		ctx:              ctx,
		view:             &view.View{Measure: measure, TagKeys: tagKeys, Aggregation: aggregation},
		incrementMeasure: []stats.Measurement{measure.M(1)},
		decrementMeasure: []stats.Measurement{measure.M(-1)},
	}
}

func (f *float64Metric) Increment() {
	f.recordMeasurements(f.incrementMeasure)
}

func (f *float64Metric) Decrement() {
	f.recordMeasurements(f.decrementMeasure)
}

func (f *float64Metric) Name() string {
	return f.Float64Measure.Name()
}

func (f *float64Metric) Record(value float64) {
	recordHookMutex.RLock()
	if rh, ok := recordHooks[f.Name()]; ok {
		rh.OnRecordFloat64Measure(f.Float64Measure, f.tags, value)
	}
	recordHookMutex.RUnlock()
	m := f.M(value)
	stats.Record(f.ctx, m) //nolint:errcheck
}

func (f *float64Metric) recordMeasurements(m []stats.Measurement) {
	recordHookMutex.RLock()
	if rh, ok := recordHooks[f.Name()]; ok {
		for _, mv := range m {
			rh.OnRecordFloat64Measure(f.Float64Measure, f.tags, mv.Value())
		}
	}
	recordHookMutex.RUnlock()
	stats.Record(f.ctx, m...)
}

func (f *float64Metric) RecordInt(value int64) {
	f.Record(float64(value))
}

func (f *float64Metric) With(labelValues ...LabelValue) Metric {
	t := make([]tag.Mutator, len(f.tags), len(f.tags)+len(labelValues))
	copy(t, f.tags)
	for _, tagValue := range labelValues {
		t = append(t, tag.Mutator(tagValue))
	}
	ctx, _ := tag.New(context.Background(), t...) //nolint:errcheck
	return &float64Metric{
		Float64Measure:   f.Float64Measure,
		tags:             t,
		ctx:              ctx,
		view:             f.view,
		incrementMeasure: f.incrementMeasure,
		decrementMeasure: f.decrementMeasure,
	}
}

func (f *float64Metric) Register() error {
	return view.Register(f.view)
}

type int64Metric struct {
	*stats.Int64Measure

	// tags stores all tags for the metrics
	tags []tag.Mutator
	// ctx is a precomputed context holding tags, as an optimization
	ctx  context.Context
	view *view.View

	// incrementMeasure is a precomputed +1 measurement to avoid extra allocations in Increment()
	incrementMeasure []stats.Measurement
	// decrementMeasure is a precomputed -1 measurement to avoid extra allocations in Decrement()
	decrementMeasure []stats.Measurement
}

func newInt64Metric(name, description string, aggregation *view.Aggregation, opts *options) *int64Metric {
	measure := stats.Int64(name, description, string(opts.unit))
	tagKeys := make([]tag.Key, 0, len(opts.labels))
	for _, l := range opts.labels {
		tagKeys = append(tagKeys, tag.Key(l))
	}
	ctx, _ := tag.New(context.Background()) //nolint:errcheck
	return &int64Metric{
		Int64Measure:     measure,
		tags:             make([]tag.Mutator, 0),
		ctx:              ctx,
		view:             &view.View{Measure: measure, TagKeys: tagKeys, Aggregation: aggregation},
		incrementMeasure: []stats.Measurement{measure.M(1)},
		decrementMeasure: []stats.Measurement{measure.M(-1)},
	}
}

func (i *int64Metric) Increment() {
	i.recordMeasurements(i.incrementMeasure)
}

func (i *int64Metric) Decrement() {
	i.recordMeasurements(i.decrementMeasure)
}

func (i *int64Metric) Name() string {
	return i.Int64Measure.Name()
}

func (i *int64Metric) Record(value float64) {
	i.RecordInt(int64(math.Floor(value)))
}

func (i *int64Metric) recordMeasurements(m []stats.Measurement) {
	recordHookMutex.RLock()
	if rh, ok := recordHooks[i.Name()]; ok {
		for _, mv := range m {
			rh.OnRecordInt64Measure(i.Int64Measure, i.tags, int64(math.Floor(mv.Value())))
		}
	}
	recordHookMutex.RUnlock()
	stats.Record(i.ctx, m...) //nolint:errcheck
}

func (i *int64Metric) RecordInt(value int64) {
	recordHookMutex.RLock()
	if rh, ok := recordHooks[i.Name()]; ok {
		rh.OnRecordInt64Measure(i.Int64Measure, i.tags, value)
	}
	recordHookMutex.RUnlock()
	stats.Record(i.ctx, i.M(value)) //nolint:errcheck
}

func (i *int64Metric) With(labelValues ...LabelValue) Metric {
	t := make([]tag.Mutator, len(i.tags), len(i.tags)+len(labelValues))
	copy(t, i.tags)
	for _, tagValue := range labelValues {
		t = append(t, tag.Mutator(tagValue))
	}
	ctx, _ := tag.New(context.Background(), t...) //nolint:errcheck
	return &int64Metric{
		Int64Measure:     i.Int64Measure,
		tags:             t,
		ctx:              ctx,
		view:             i.view,
		incrementMeasure: i.incrementMeasure,
		decrementMeasure: i.decrementMeasure,
	}
}

func (i *int64Metric) Register() error {
	return view.Register(i.view)
}
