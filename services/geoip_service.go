package services

import (
	"net"
	"net/netip"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

type Service struct {
	city *geoip2.Reader
	asn  *geoip2.Reader
}

type Info struct {
	CountryISO string  `json:"country_iso"`
	Country    string  `json:"country"`
	Region     string  `json:"region"`
	City       string  `json:"city"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lon"`
	Timezone   string  `json:"timezone"`
	ASN        uint    `json:"asn"`
	ASOrg      string  `json:"as_org"`
}

func NewGeoSerice(cityDBPath, asnDBPath string) (*Service, error) {
	c, err := geoip2.Open(cityDBPath)
	if err != nil {
		return nil, err
	}
	a, err := geoip2.Open(asnDBPath)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	return &Service{city: c, asn: a}, nil
}

func (s *Service) Close() error {
	var err1, err2 error
	if s.city != nil {
		err1 = s.city.Close()
	}
	if s.asn != nil {
		err2 = s.asn.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}

// 传入字符串 IP；内部同时兼容 IPv4/IPv6；私网/无效地址直接报错
func (s *Service) Lookup(ipStr string) (*Info, error) {
	ipStr = strings.TrimSpace(ipStr)
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, err
	}
	/*if addr.IsPrivate() || addr.IsLoopback() || addr.IsUnspecified() {
		return nil, errors.New("non-public ip")
	}*/

	// geoip2 接口仍用 net.IP
	ip := net.ParseIP(addr.String())

	cityRec, err := s.city.City(ip)
	if err != nil {
		return nil, err
	}
	asnRec, err := s.asn.ASN(ip)
	if err != nil {
		return nil, err
	}

	info := &Info{
		CountryISO: cityRec.Country.IsoCode,
		Country:    cityRec.Country.Names["en"],
		City:       cityRec.City.Names["en"],
		Timezone:   cityRec.Location.TimeZone,
		Latitude:   cityRec.Location.Latitude,
		Longitude:  cityRec.Location.Longitude,
		ASN:        asnRec.AutonomousSystemNumber,
		ASOrg:      asnRec.AutonomousSystemOrganization,
	}
	// 省/州（可能为空）
	if len(cityRec.Subdivisions) > 0 {
		info.Region = cityRec.Subdivisions[0].Names["en"]
	}
	return info, nil
}
