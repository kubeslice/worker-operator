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

package gatewayedge

import (
	"context"

	edgeproto "github.com/kubeslice/slicegw-edge/pkg/edgeservice"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type gwEdgeClient struct {
}

type SliceGwServiceInfo struct {
	edgeproto.SliceGwServiceInfo
}

type SliceGwServiceMap struct {
	edgeproto.SliceGwServiceMap
}

type GwEdgeResponse struct {
	edgeproto.GwEdgeResponse
}

func NewWorkerGatewayEdgeClientProvider() (*gwEdgeClient, error) {
	return &gwEdgeClient{}, nil
}

func (e gwEdgeClient) UpdateSliceGwServiceMap(ctx context.Context, serverAddr string, gwSvcMap *SliceGwServiceMap) (*GwEdgeResponse, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := edgeproto.NewGwEdgeServiceClient(conn)
	_, err = client.UpdateSliceGwServiceMap(ctx, &gwSvcMap.SliceGwServiceMap)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
