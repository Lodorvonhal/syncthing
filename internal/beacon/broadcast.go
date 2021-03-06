// Copyright (C) 2014 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

package beacon

import "net"

type Broadcast struct {
	conn   *net.UDPConn
	port   int
	inbox  chan []byte
	outbox chan recv
}

func NewBroadcast(port int) (*Broadcast, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: port})
	if err != nil {
		return nil, err
	}
	b := &Broadcast{
		conn:   conn,
		port:   port,
		inbox:  make(chan []byte),
		outbox: make(chan recv, 16),
	}

	go genericReader(b.conn, b.outbox)
	go b.writer()

	return b, nil
}

func (b *Broadcast) Send(data []byte) {
	b.inbox <- data
}

func (b *Broadcast) Recv() ([]byte, net.Addr) {
	recv := <-b.outbox
	return recv.data, recv.src
}

func (b *Broadcast) writer() {
	for bs := range b.inbox {

		addrs, err := net.InterfaceAddrs()
		if err != nil {
			if debug {
				l.Debugln("Broadcast: interface addresses:", err)
			}
			continue
		}

		var dsts []net.IP
		for _, addr := range addrs {
			if iaddr, ok := addr.(*net.IPNet); ok && len(iaddr.IP) >= 4 && iaddr.IP.IsGlobalUnicast() && iaddr.IP.To4() != nil {
				baddr := bcast(iaddr)
				dsts = append(dsts, baddr.IP)
			}
		}

		if len(dsts) == 0 {
			// Fall back to the general IPv4 broadcast address
			dsts = append(dsts, net.IP{0xff, 0xff, 0xff, 0xff})
		}

		if debug {
			l.Debugln("addresses:", dsts)
		}

		for _, ip := range dsts {
			dst := &net.UDPAddr{IP: ip, Port: b.port}

			_, err := b.conn.WriteTo(bs, dst)
			if err != nil {
				if debug {
					l.Debugln(err)
				}
			} else if debug {
				l.Debugf("sent %d bytes to %s", len(bs), dst)
			}
		}
	}
}

func bcast(ip *net.IPNet) *net.IPNet {
	var bc = &net.IPNet{}
	bc.IP = make([]byte, len(ip.IP))
	copy(bc.IP, ip.IP)
	bc.Mask = ip.Mask

	offset := len(bc.IP) - len(bc.Mask)
	for i := range bc.IP {
		if i-offset >= 0 {
			bc.IP[i] = ip.IP[i] | ^ip.Mask[i-offset]
		}
	}
	return bc
}
