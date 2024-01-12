package redis

import "context"

func (rr *RedisReconciler) UpdateInstanceStatus() error {
	return rr.Client.Status().Update(context.TODO(), rr.Instance)
}
