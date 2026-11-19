package logic

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/pkg/objectstorage"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// tmpSweepInterval 后台清理 tmp 残留的轮询周期。
	tmpSweepInterval = 15 * time.Minute
	// tmpObjectMaxAge tmp 对象存活上限：远超 presigned PUT TTL（uploadURLTTL）仍未确认即视为
	// 中断/失败上传留下的孤儿，可回收。
	tmpObjectMaxAge = time.Hour
)

// TempUploadCleaner 周期性回收 tmp/ 前缀下中断/失败上传留下的残留对象（EPIC #527 §3）。正常链路
// confirm 成功即 copy+delete 删掉 tmp；此清理兜底未确认的孤儿对象，按对象 LastModified 年龄判定。
type TempUploadCleaner struct {
	store  objectstorage.ObjectStore
	maxAge time.Duration
	now    func() time.Time
}

func NewTempUploadCleaner(store objectstorage.ObjectStore) *TempUploadCleaner {
	return &TempUploadCleaner{store: store, maxAge: tmpObjectMaxAge, now: time.Now}
}

// Run 阻塞轮询直到 ctx 取消；每周期跑一次 SweepOnce，单次错误只记日志不退出。
func (c *TempUploadCleaner) Run(ctx context.Context) {
	ticker := time.NewTicker(tmpSweepInterval)
	defer ticker.Stop()
	for {
		if removed, err := c.SweepOnce(ctx); err != nil {
			logx.WithContext(ctx).Errorf("tmp upload sweep failed: %v", err)
		} else if removed > 0 {
			logx.WithContext(ctx).Infof("tmp upload sweep removed %d stale object(s)", removed)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// SweepOnce 列出 tmp/ 下 LastModified 早于 now-maxAge 的对象并删除，返回删除数。
func (c *TempUploadCleaner) SweepOnce(ctx context.Context) (int, error) {
	objects, err := c.store.ListByPrefix(ctx, tmpKeyPrefix)
	if err != nil {
		return 0, err
	}
	cutoff := c.now().Add(-c.maxAge)
	removed := 0
	for _, obj := range objects {
		if obj.LastModified.After(cutoff) {
			continue
		}
		if err := c.store.RemoveObject(ctx, obj.ObjectKey); err != nil {
			logx.WithContext(ctx).Errorf("tmp upload sweep: remove %q failed: %v", obj.ObjectKey, err)
			continue
		}
		removed++
	}
	return removed, nil
}
