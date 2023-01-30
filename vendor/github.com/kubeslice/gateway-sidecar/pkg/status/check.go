/*  Copyright (c) 2022 Avesha, Inc. All rights reserved.
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

package status

type Check interface {
	// Execute runs a single time check, and returns execution status.
	Execute(interface{}) (err error)
	// MessageHandler handles the message and returns message handling status.
	MessageHandler(msg interface{}) (err error)
	// Status to provide the status of the check
	Status() (details *TunnelInterfaceStatus, err error)
	// Stop the status of the check
	Stop() (err error)
}
