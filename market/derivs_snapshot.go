package market

import (
	"log"
	"sync"

	configpkg "nofx/config"
	derivsv1 "nofx/internal/features/derivs/v1"
	"nofx/internal/snapshot"
)

var (
	snapshotOnce sync.Once
	snapshotBld  *snapshot.Builder
)

func ensureSnapshotBuilder() {
	snapshotOnce.Do(func() {
		cfg, err := configpkg.LoadDerivsV1Config("config/derivs_v1.yaml")
		if err != nil {
			log.Printf("⚠️ 载入衍生品特征配置失败: %v", err)
			return
		}
		if cfg == nil || !cfg.Enabled {
			log.Printf("⚠️ [Derivs] Config missing or disabled")
			return
		}
		store, err := derivsv1.NewRedisStore(derivsv1.RedisOptions{
			Addr:     cfg.Cache.RedisAddr,
			Password: cfg.Cache.RedisPassword,
			DB:       cfg.Cache.RedisDB,
			Keyspace: cfg.Cache.Keyspace,
		})
		if err != nil {
			log.Printf("⚠️ 初始化 Redis 衍生品缓存失败: %v", err)
			return
		}
		snapshotBld = snapshot.NewBuilder(cfg, store)
	})
}

func buildDerivsSnapshot(symbol string) *snapshot.Snapshot {
	ensureSnapshotBuilder()
	if snapshotBld == nil {
		return nil
	}
	snap, err := snapshotBld.Build(symbol)
	if err != nil {
		log.Printf("⚠️ 构建衍生品特征失败 (%s): %v", symbol, err)
		return nil
	}
	if snap == nil || snap.Features.Derivs == nil {
		return nil
	}
	return snap
}
