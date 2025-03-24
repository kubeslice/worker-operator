/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1beta2

// LimitedPriorityLevelConfigurationApplyConfiguration represents a declarative configuration of the LimitedPriorityLevelConfiguration type for use
// with apply.
type LimitedPriorityLevelConfigurationApplyConfiguration struct {
	AssuredConcurrencyShares *int32                           `json:"assuredConcurrencyShares,omitempty"`
	LimitResponse            *LimitResponseApplyConfiguration `json:"limitResponse,omitempty"`
	LendablePercent          *int32                           `json:"lendablePercent,omitempty"`
	BorrowingLimitPercent    *int32                           `json:"borrowingLimitPercent,omitempty"`
}

// LimitedPriorityLevelConfigurationApplyConfiguration constructs a declarative configuration of the LimitedPriorityLevelConfiguration type for use with
// apply.
func LimitedPriorityLevelConfiguration() *LimitedPriorityLevelConfigurationApplyConfiguration {
	return &LimitedPriorityLevelConfigurationApplyConfiguration{}
}

// WithAssuredConcurrencyShares sets the AssuredConcurrencyShares field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the AssuredConcurrencyShares field is set to the value of the last call.
func (b *LimitedPriorityLevelConfigurationApplyConfiguration) WithAssuredConcurrencyShares(value int32) *LimitedPriorityLevelConfigurationApplyConfiguration {
	b.AssuredConcurrencyShares = &value
	return b
}

// WithLimitResponse sets the LimitResponse field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the LimitResponse field is set to the value of the last call.
func (b *LimitedPriorityLevelConfigurationApplyConfiguration) WithLimitResponse(value *LimitResponseApplyConfiguration) *LimitedPriorityLevelConfigurationApplyConfiguration {
	b.LimitResponse = value
	return b
}

// WithLendablePercent sets the LendablePercent field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the LendablePercent field is set to the value of the last call.
func (b *LimitedPriorityLevelConfigurationApplyConfiguration) WithLendablePercent(value int32) *LimitedPriorityLevelConfigurationApplyConfiguration {
	b.LendablePercent = &value
	return b
}

// WithBorrowingLimitPercent sets the BorrowingLimitPercent field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the BorrowingLimitPercent field is set to the value of the last call.
func (b *LimitedPriorityLevelConfigurationApplyConfiguration) WithBorrowingLimitPercent(value int32) *LimitedPriorityLevelConfigurationApplyConfiguration {
	b.BorrowingLimitPercent = &value
	return b
}
