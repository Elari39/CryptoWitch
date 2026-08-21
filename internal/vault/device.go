package vault

import (
	"net"
	"strings"
)

// deviceWildcard 表示跳过 MAC 校验的通配符。
const deviceWildcard = "*"

// normalizeMAC 将 MAC 地址规范化为无分隔符的小写形式，便于忽略格式差异比对。
// 例如 "AA-BB.CC:DD:ee:FF" -> "aabbccddeeff"（去除 :, -, .）。
func normalizeMAC(mac string) string {
	trimmed := strings.ToLower(strings.TrimSpace(mac))
	if trimmed == deviceWildcard {
		return deviceWildcard
	}
	trimmed = strings.ReplaceAll(trimmed, ":", "")
	trimmed = strings.ReplaceAll(trimmed, "-", "")
	trimmed = strings.ReplaceAll(trimmed, ".", "")
	return trimmed
}

// localMACAddresses 返回本机所有非空网卡 MAC 地址（已规范化）。
func localMACAddresses() ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var addresses []string
	for _, iface := range interfaces {
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		addresses = append(addresses, normalizeMAC(iface.HardwareAddr.String()))
	}
	return addresses, nil
}

// verifyDevice 校验本机网卡 MAC 是否命中白名单。
// 白名单为空或含 "*" 时跳过校验（向后兼容：未配置即仅凭密码访问）。
// 否则本机任一 MAC 命中白名单即放行。
//
// 取舍说明：设备校验失败不累计 unlockFailures（不触发限速冷却）。因为白名单
// 本就以明文形式存在于二进制中（见 README「安全说明」），MAC 校验仅是访问增强
// 而非机密边界，对设备拒绝做限速没有额外的安全收益。
func (s *Service) verifyDevice() error {
	allowed := make(map[string]struct{}, len(s.allowedMACs))
	for _, mac := range s.allowedMACs {
		normalized := normalizeMAC(mac)
		if normalized == deviceWildcard || normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}
	// 未配置具体 MAC（空白名单或仅含通配符/空值）时不启用设备校验。
	if len(allowed) == 0 {
		return nil
	}

	local, err := localMACAddresses()
	if err != nil {
		return ErrDeviceNotAuthorized
	}
	for _, mac := range local {
		if _, ok := allowed[mac]; ok {
			return nil
		}
	}
	return ErrDeviceNotAuthorized
}
