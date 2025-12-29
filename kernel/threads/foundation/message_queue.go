package foundation

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"unsafe"
)

// Message queue constants
const (
	MESSAGE_SIZE         = 256
	MESSAGE_HEADER_SIZE  = 32
	MESSAGE_PAYLOAD_SIZE = 224
	MESSAGE_MAGIC        = 0x4D53475F45504F43
)

// MessageQueue implements a zero-copy ring buffer in SAB
type MessageQueue struct {
	sab        []byte
	baseOffset uint32
	capacity   uint32
	headOffset uint32
	tailOffset uint32
	stats      QueueStats
}

// QueueStats tracks queue performance
type QueueStats struct {
	Enqueued   uint64
	Dequeued   uint64
	Dropped    uint64
	QueueDepth uint32
	MaxDepth   uint32
}

// MessageHeader represents the message header
type MessageHeader struct {
	Magic         uint64
	Sequence      uint64
	MsgType       uint8
	Priority      uint8
	SenderEpoch   uint8
	ReceiverEpoch uint8
	Flags         uint16
	DataSize      uint16
	Checksum      uint16
}

// NewMessageQueue creates a new message queue
func NewMessageQueue(sab []byte, baseOffset, capacity uint32) *MessageQueue {
	if capacity&(capacity-1) != 0 {
		panic("capacity must be power of 2")
	}
	headOffset := baseOffset - 8
	tailOffset := baseOffset - 4
	return &MessageQueue{
		sab:        sab,
		baseOffset: baseOffset,
		capacity:   capacity,
		headOffset: headOffset,
		tailOffset: tailOffset,
	}
}

func (mq *MessageQueue) EnqueueZeroCopy(msgType, priority uint8, dataSize uint16) (uint32, error) {
	if dataSize > MESSAGE_PAYLOAD_SIZE {
		return 0, fmt.Errorf("data size exceeds max payload")
	}
	tail := atomic.LoadUint32((*uint32)(unsafe.Pointer(&mq.sab[mq.tailOffset])))
	nextTail := (tail + 1) & (mq.capacity - 1)
	head := atomic.LoadUint32((*uint32)(unsafe.Pointer(&mq.sab[mq.headOffset])))
	if nextTail == head {
		atomic.AddUint64(&mq.stats.Dropped, 1)
		return 0, fmt.Errorf("queue full")
	}
	msgOffset := mq.baseOffset + (tail * MESSAGE_SIZE)
	header := MessageHeader{
		Magic:    MESSAGE_MAGIC,
		Sequence: atomic.AddUint64(&mq.stats.Enqueued, 1),
		MsgType:  msgType,
		Priority: priority,
		DataSize: dataSize,
	}
	mq.writeHeader(msgOffset, &header)
	atomic.StoreUint32((*uint32)(unsafe.Pointer(&mq.sab[mq.tailOffset])), nextTail)
	return msgOffset + MESSAGE_HEADER_SIZE, nil
}

func (mq *MessageQueue) DequeueZeroCopy() (uint8, uint16, uint32, error) {
	head := atomic.LoadUint32((*uint32)(unsafe.Pointer(&mq.sab[mq.headOffset])))
	tail := atomic.LoadUint32((*uint32)(unsafe.Pointer(&mq.sab[mq.tailOffset])))
	if head == tail {
		return 0, 0, 0, fmt.Errorf("queue empty")
	}
	msgOffset := mq.baseOffset + (head * MESSAGE_SIZE)
	header := mq.readHeader(msgOffset)
	if header.Magic != MESSAGE_MAGIC {
		return 0, 0, 0, fmt.Errorf("corrupted message")
	}
	nextHead := (head + 1) & (mq.capacity - 1)
	atomic.StoreUint32((*uint32)(unsafe.Pointer(&mq.sab[mq.headOffset])), nextHead)
	atomic.AddUint64(&mq.stats.Dequeued, 1)
	return header.MsgType, header.DataSize, msgOffset + MESSAGE_HEADER_SIZE, nil
}

// FinalizeMessage finalizing a message by updating its header (e.g. checksum)
func (mq *MessageQueue) FinalizeMessage(headerOffset uint32, data []byte) {
	// Calculate checksum
	var checksum uint16
	for _, b := range data {
		checksum += uint16(b)
	}

	// Update checksum in header at offset+24
	binary.LittleEndian.PutUint16(mq.sab[headerOffset+24:], checksum)
}

func (mq *MessageQueue) writeHeader(offset uint32, header *MessageHeader) {
	binary.LittleEndian.PutUint64(mq.sab[offset:], header.Magic)
	binary.LittleEndian.PutUint64(mq.sab[offset+8:], header.Sequence)
	mq.sab[offset+16] = header.MsgType
	mq.sab[offset+17] = header.Priority
	mq.sab[offset+18] = header.SenderEpoch
	mq.sab[offset+19] = header.ReceiverEpoch
	binary.LittleEndian.PutUint16(mq.sab[offset+20:], header.Flags)
	binary.LittleEndian.PutUint16(mq.sab[offset+22:], header.DataSize)
	binary.LittleEndian.PutUint16(mq.sab[offset+24:], header.Checksum)
}

func (mq *MessageQueue) readHeader(offset uint32) MessageHeader {
	return MessageHeader{
		Magic:    binary.LittleEndian.Uint64(mq.sab[offset:]),
		Sequence: binary.LittleEndian.Uint64(mq.sab[offset+8:]),
		MsgType:  mq.sab[offset+16],
		Priority: mq.sab[offset+17],
		DataSize: binary.LittleEndian.Uint16(mq.sab[offset+22:]),
	}
}
