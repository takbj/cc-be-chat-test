package app

import ()

type TClient struct {
}

func NewClient() *TClient {
	return &TClient{}
}

func (c *TClient) Connect(serverAddr string) {

}
