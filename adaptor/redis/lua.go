package redis

import "github.com/go-redis/redis"

var luaRenew = redis.NewScript(`
		local k = KEYS[1]
		local v = ARGV[1]
		if redis.call("GET", k) == v then
		  return redis.call("EXPIRE", k, ARGV[2])
		else
		  return 0
		end
`)
var luaGetAndDelete = redis.NewScript(`
	local v = redis.call("GET",KEYS[1]) 
	if v then
		redis.call("DEL",KEYS[1])
	end
	return v
`)
var luaUnlock = redis.NewScript(`
		local k = KEYS[1]
		local v = ARGV[1]
		if redis.call("GET", k) == v then
		  return redis.call("DEL", k)
		end
`)
