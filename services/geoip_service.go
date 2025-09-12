package services

import (
	"net"
	"net/netip"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

type GeoService struct {
	city *geoip2.Reader
	asn  *geoip2.Reader
}

type IPInfo struct {
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

func NewGeoSerice(cityDBPath, asnDBPath string) (*GeoService, error) {
	c, err := geoip2.Open(cityDBPath)
	if err != nil {
		return nil, err
	}
	a, err := geoip2.Open(asnDBPath)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	return &GeoService{city: c, asn: a}, nil
}

func (s *GeoService) Close() error {
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

// Lookup 传入字符串 IP；内部同时兼容 IPv4/IPv6；私网/无效地址直接报错
func (s *GeoService) Lookup(ipStr string) (*IPInfo, error) {
	ipStr = strings.TrimSpace(ipStr)
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return nil, err
	}

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

	s2 := "zh-CN"
	info := &IPInfo{
		CountryISO: cityRec.Country.IsoCode,
		Country:    cityRec.Country.Names[s2],
		City:       cityRec.City.Names[s2],
		Timezone:   cityRec.Location.TimeZone,
		Latitude:   cityRec.Location.Latitude,
		Longitude:  cityRec.Location.Longitude,
		ASN:        asnRec.AutonomousSystemNumber,
		ASOrg:      asnRec.AutonomousSystemOrganization,
	}
	// 省/州（可能为空）
	if len(cityRec.Subdivisions) > 0 {
		info.Region = cityRec.Subdivisions[0].Names[s2]
	}
	return info, nil
}
