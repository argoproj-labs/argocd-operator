/*
Copyright 2019, 2021.

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

package v1alpha1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateArgoCDCR(t *testing.T) {

	tests := []struct {
		dynamicScalingEnabled bool
		minShards             int32
		maxShards             int32
		clustersPerShard      int32
		err                   error
	}{
		{
			dynamicScalingEnabled: false,
			minShards:             1,
			maxShards:             2,
			clustersPerShard:      2,
			err:                   nil,
		},
		{
			dynamicScalingEnabled: true,
			minShards:             -1,
			maxShards:             2,
			clustersPerShard:      2,
			err:                   fmt.Errorf("spec.controller.sharding.minShards cannot be less than 1"),
		},
		{
			dynamicScalingEnabled: true,
			minShards:             1,
			maxShards:             2,
			clustersPerShard:      -1,
			err:                   fmt.Errorf("spec.controller.sharding.clustersPerShard cannot be less than 1"),
		},
		{
			dynamicScalingEnabled: true,
			minShards:             2,
			maxShards:             1,
			clustersPerShard:      2,
			err:                   fmt.Errorf("spec.controller.sharding.maxShards cannot be less than spec.controller.sharding.minShards"),
		},
	}

	for _, test := range tests {
		cr := &ArgoCD{
			Spec: ArgoCDSpec{
				Controller: ArgoCDApplicationControllerSpec{
					Sharding: ArgoCDApplicationControllerShardSpec{
						DynamicScalingEnabled: test.dynamicScalingEnabled,
						MinShards:             test.minShards,
						MaxShards:             test.maxShards,
						ClustersPerShard:      test.clustersPerShard,
					},
				},
			},
		}

		err := cr.ValidateArgocdCR()
		assert.Equal(t, test.err, err)
	}

}
