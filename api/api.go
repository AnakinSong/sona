package api

import (
    "time"
    "errors"
    "strings"
    "sona/core"
    "sona/protocol"
    "sona/common/net/udp/client"
    "github.com/golang/protobuf/proto"
)

type SonaApi struct {
    getter *core.ConfigGetter
    serviceKey string
    udpClient *client.Client
}

func GetApi(serviceKey string) (*SonaApi, error) {
    serviceKey = strings.Trim(serviceKey, " ")
    if !core.IsValidityServiceKey(serviceKey) {
        return nil, errors.New("not a valid sona service key")
    }
    api := SonaApi{}
    //create UDP client
    udpClient, err := client.CreateClient("127.0.0.1", 9901)
    if err != nil {
        return nil, err
    }
    api.udpClient = udpClient
    //订阅
    index, err := api.subscribe(serviceKey)
    if err != nil {
        //订阅错误
        return nil, err
    }
    //订阅成功，创建getter
    getter, err := core.GetConfigGetter(serviceKey, index)
    if err != nil {
        return nil, err
    }
    api.getter = getter
    api.serviceKey = serviceKey
    api.subscribe(serviceKey)
    //启动一个保活routine:告诉agent我一直在使用此serviceKey
    go api.keepUsing()
    return &api, nil
}

func (api *SonaApi) keepUsing() {
    req := &protocol.KeepUsingReq{ServiceKey:proto.String(api.serviceKey)}
    for {
        time.Sleep(time.Second * 60)
        //tell agent
        api.udpClient.Send(protocol.KeepUsingReqId, req)
    }
}

//向agent发起订阅，返回值是：索引
func (api *SonaApi) subscribe(serviceKey string) (uint, error) {
    if !core.IsValidityServiceKey(serviceKey) {
        return core.ServiceBucketLimit, errors.New("service key format error")
    }
    //发起订阅消息
    req := &protocol.SubscribeReq{}
    req.ServiceKey = proto.String(serviceKey)
    err := api.udpClient.Send(protocol.SubscribeReqId, req)
    if err != nil {
        return core.ServiceBucketLimit, err
    }

    rsp := protocol.SubscribeAgentRsp{}
    //接收 300ms超时
    timeout := 300 * time.Millisecond
    err = api.udpClient.Read(timeout, protocol.SubscribeAgentRspId, &rsp)
    if err != nil {
        return core.ServiceBucketLimit, err
    }
    //收到包，处理
    if *rsp.ServiceKey != serviceKey {
        return core.ServiceBucketLimit, errors.New("udp receive error data")
    }
    //订阅失败
    if *rsp.Code == -1 {
        return core.ServiceBucketLimit, errors.New("no such a service in system right now")
    }
    //订阅成功，获取版本
    index := *rsp.Index
    return uint(index), nil
}

//获取value
func (api *SonaApi) Get(section string, key string) string {
    section = strings.Trim(section, " ")
    key = strings.Trim(key, " ")
    confKey := section + "." + key
    if !core.IsValidityConfKey(confKey) {
        return ""
    }
    return api.getter.Get(confKey)
}

//获取value并以列表解析
func (api *SonaApi) GetList(section string, key string) []string {
    confKey := section + "." + key
    if !core.IsValidityConfKey(confKey) {
        return nil
    }
    value := api.getter.Get(confKey)
    items := strings.Split(value, ",")
    for idx, item := range items {
        items[idx] = strings.TrimSpace(item)
    }
    return items
}

func (api *SonaApi) Close() {
    api.getter.Close()
    api.udpClient.Close()
}
