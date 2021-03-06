package mesh

import (
	"errors"
	"github.com/spacemeshos/go-spacemesh/database"
	"github.com/spacemeshos/go-spacemesh/log"
	"sync"
	"sync/atomic"
)

const layerSize = 200
const cachedLayers = 50

type Mesh interface {
	AddLayer(layer *Layer) error
	GetLayer(i LayerID) (*Layer, error)
	GetBlock(id BlockID) (*Block, error)
	AddBlock(block *Block) error
	GetContextualValidity(id BlockID) (bool, error)
	LatestIrreversible() uint32
	LatestKnownLayer() uint32
	SetLatestKnownLayer(idx uint32)
	Close()
}

type mesh struct {
	latestIrreversible uint32
	latestKnownLayer   uint32
	mDB                *meshDB
	lMutex             sync.RWMutex
	lkMutex            sync.RWMutex
	lcMutex            sync.RWMutex
	tortoise           Algorithm
}

func NewMesh(layers database.DB, blocks database.DB, validity database.DB) Mesh {
	//todo add boot from disk
	ll := &mesh{
		tortoise: NewAlgorithm(uint32(layerSize), uint32(cachedLayers)),
		mDB:      NewMeshDb(layers, blocks, validity),
	}
	return ll
}

func (m *mesh) IsContexuallyValid(b BlockID) bool {
	//todo implement
	return true
}

func (m *mesh) LatestIrreversible() uint32 {
	return atomic.LoadUint32(&m.latestIrreversible)
}

func (m *mesh) LatestKnownLayer() uint32 {
	defer m.lkMutex.RUnlock()
	m.lkMutex.RLock()
	return m.latestKnownLayer
}

func (m *mesh) SetLatestKnownLayer(idx uint32) {
	defer m.lkMutex.Unlock()
	m.lkMutex.Lock()
	if idx > m.latestKnownLayer {
		log.Debug("set latest known layer to ", idx)
		m.latestKnownLayer = idx
	}
}

func (m *mesh) AddLayer(layer *Layer) error {
	m.lMutex.Lock()
	defer m.lMutex.Unlock()
	count := LayerID(m.LatestIrreversible())
	if count > layer.Index() {
		log.Debug("can't add layer ", layer.Index(), "(already exists)")
		return errors.New("can't add layer (already exists)")
	}

	if count < layer.Index() {
		log.Debug("can't add layer", layer.Index(), " missing previous layers")
		return errors.New("can't add layer missing previous layers")
	}

	m.mDB.addLayer(layer)
	m.tortoise.HandleIncomingLayer(layer)
	atomic.AddUint32(&m.latestIrreversible, 1)
	return nil
}

func (m *mesh) GetLayer(i LayerID) (*Layer, error) {
	m.lMutex.RLock()
	if i >= LayerID(m.latestIrreversible) {
		m.lMutex.RUnlock()
		log.Debug("failed to get layer  ", i, " layer not verified yet")
		return nil, errors.New("layer not verified yet")
	}
	m.lMutex.RUnlock()
	return m.mDB.getLayer(i)
}

func (m *mesh) AddBlock(block *Block) error {
	log.Debug("add block ", block.ID())
	if err := m.mDB.addBlock(block); err != nil {
		log.Debug("failed to add block ", block.ID(), " ", err)
		return err
	}
	m.SetLatestKnownLayer(uint32(block.Layer()))
	m.tortoise.HandleLateBlock(block) //todo should be thread safe?
	return nil
}

func (m *mesh) GetBlock(id BlockID) (*Block, error) {
	log.Debug("get block ", id)
	return m.mDB.getBlock(id)
}

func (m *mesh) GetContextualValidity(id BlockID) (bool, error) {
	return m.mDB.getContextualValidity(id)
}

func (m *mesh) Close() {
	log.Debug("closing mDB")
	m.mDB.Close()
}
