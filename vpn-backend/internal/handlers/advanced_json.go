package handlers

import (
	"encoding/json"
)

func applyAdvancedJSON(parsed map[string]interface{}, advancedJSON string, isServerConfig bool) {
	if advancedJSON == "" || advancedJSON == "{}" {
		return
	}
	var adv map[string]interface{}
	if err := json.Unmarshal([]byte(advancedJSON), &adv); err != nil {
		return
	}

	_, hasProtocol := adv["protocol"]
	_, hasListen := adv["listen"]
	_, isRootInbounds := adv["inbounds"]
	_, isRootOutbounds := adv["outbounds"]

	if !isRootInbounds && !isRootOutbounds && (hasProtocol || hasListen) {
		if isServerConfig {
			// Merge into inbounds[0] for server
			if inboundsRaw, ok := parsed["inbounds"].([]interface{}); ok && len(inboundsRaw) > 0 {
				if inbound, ok := inboundsRaw[0].(map[string]interface{}); ok {
					deepMergeMap(inbound, adv)
					return
				}
			}
		} else {
			// Merge into outbounds[0] for client
			if outboundsRaw, ok := parsed["outbounds"].([]interface{}); ok && len(outboundsRaw) > 0 {
				if outbound, ok := outboundsRaw[0].(map[string]interface{}); ok {
					deepMergeMap(outbound, adv)
					return
				}
			}
		}
	}

	if isServerConfig {
		delete(adv, "outbounds")
	} else {
		delete(adv, "inbounds")
	}

	deepMergeMap(parsed, adv)
}

func deepMergeMap(dst, src map[string]interface{}) {
	for k, v := range src {
		if vMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := dst[k].(map[string]interface{}); ok {
				deepMergeMap(dstMap, vMap)
				continue
			}
		} else if vArr, ok := v.([]interface{}); ok {
			if dstArr, ok := dst[k].([]interface{}); ok && len(dstArr) > 0 && len(vArr) > 0 {
				if vMap, ok := vArr[0].(map[string]interface{}); ok {
					if dstMap, ok := dstArr[0].(map[string]interface{}); ok {
						deepMergeMap(dstMap, vMap)
						for i := 1; i < len(vArr); i++ {
							dstArr = append(dstArr, vArr[i])
						}
						dst[k] = dstArr
						continue
					}
				}
			}
		}
		dst[k] = v
	}
}
