package network

import (
	"Traceroute/utility"
	"context"
	"github.com/pkg/errors"
)

// RequestDeviceConnection represents the request device connection
type RequestDeviceConnection struct {
	RawConnectionData ConnectionData
	HTTP              *RequestDeviceConnectionHTTP
	SNMP              *RequestDeviceConnectionSNMP
}

// RequestDeviceConnectionHTTP represents the http request device connection
type RequestDeviceConnectionHTTP struct {
	HTTPClient     *HTTPClient
	ConnectionData *HTTPConnectionData
}

// RequestDeviceConnectionSNMP represents the snmp request device connection
type RequestDeviceConnectionSNMP struct {
	SnmpClient SNMPClient
	CommonOIDs CommonOIDs
}

// CommonOIDs represents the common oids
type CommonOIDs struct {
	SysObjectID    *string
	SysDescription *string
}

// GetSysDescription returns the sysDescription.
func (r *RequestDeviceConnectionSNMP) GetSysDescription(ctx context.Context) (string, error) {
	if r.CommonOIDs.SysDescription == nil {
		response, err := r.SnmpClient.SNMPGet(ctx, "1.3.6.1.2.1.1.1.0")
		if err != nil {
			return "", errors.Wrap(err, "error during snmpget")
		}
		sysDescription, err := response[0].GetValue()
		if err != nil {
			return "", errors.Wrap(err, "failed to get snmp result string")
		}
		sysDescriptionString := sysDescription.String()
		r.CommonOIDs.SysDescription = &sysDescriptionString
	}
	return *r.CommonOIDs.SysDescription, nil
}

// GetSysObjectID returns the sysObjectID.
func (r *RequestDeviceConnectionSNMP) GetSysObjectID(ctx context.Context) (string, error) {
	if r.CommonOIDs.SysObjectID == nil {
		response, err := r.SnmpClient.SNMPGet(ctx, "1.3.6.1.2.1.1.2.0")
		if err != nil {
			return "", errors.Wrap(err, "error during snmpget")
		}

		sysObjectID, err := response[0].GetValue()
		if err != nil {
			return "", errors.Wrap(err, "failed to get snmp result string")
		}
		sysObjectIDString := sysObjectID.String()
		r.CommonOIDs.SysObjectID = &sysObjectIDString
	}
	return *r.CommonOIDs.SysObjectID, nil
}

// GetIdealConnectionData returns the ideal connection data.
func (r *RequestDeviceConnection) GetIdealConnectionData() ConnectionData {
	connectionData := ConnectionData{}

	if r.SNMP != nil {
		connectionData.SNMP = &SNMPConnectionData{
			Communities: []string{r.SNMP.SnmpClient.GetCommunity()},
			Versions:    []string{r.SNMP.SnmpClient.GetVersion()},
			Ports:       []int{r.SNMP.SnmpClient.GetPort()},
			V3Data: SNMPv3ConnectionData{
				Level:        r.SNMP.SnmpClient.GetV3Level(),
				ContextName:  r.SNMP.SnmpClient.GetV3ContextName(),
				User:         r.SNMP.SnmpClient.GetV3User(),
				AuthKey:      r.SNMP.SnmpClient.GetV3AuthKey(),
				AuthProtocol: r.SNMP.SnmpClient.GetV3AuthProto(),
				PrivKey:      r.SNMP.SnmpClient.GetV3PrivKey(),
				PrivProtocol: r.SNMP.SnmpClient.GetV3PrivProto(),
			},
		}
	}

	if r.HTTP != nil {
		var null *string
		connectionData.HTTP = &HTTPConnectionData{
			AuthUsername: utility.IfThenElse(r.HTTP.HTTPClient.username == "", null, &r.HTTP.HTTPClient.username).(*string),
			AuthPassword: utility.IfThenElse(r.HTTP.HTTPClient.password == "", null, &r.HTTP.HTTPClient.password).(*string),
		}

		if r.HTTP.HTTPClient.useHTTPS {
			connectionData.HTTP.HTTPSPorts = []int{*r.HTTP.HTTPClient.port}
		} else {
			connectionData.HTTP.HTTPPorts = []int{*r.HTTP.HTTPClient.port}
		}
	}

	return connectionData
}

// CloseConnections closes the connection to the device
func (r *RequestDeviceConnection) CloseConnections() {
	if r.SNMP != nil && r.SNMP.SnmpClient != nil {
		_ = r.SNMP.SnmpClient.Disconnect()
	}
}
