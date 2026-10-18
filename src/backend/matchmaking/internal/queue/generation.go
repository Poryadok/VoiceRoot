package queue

// Generation comparison must not convert BIGINT generations to Lua doubles.
// Terminal release wins ties, while a newer persisted recovery can enqueue again.
const generationLua = `
local function valid_generation(v)
 if not v or not (v=='0' or string.match(v,'^[1-9][0-9]*$')) then return false end
 return #v<19 or (#v==19 and v<='9223372036854775807')
end
local function generation_compare(a,b)
 if #a<#b then return -1 end
 if #a>#b then return 1 end
 if a<b then return -1 end
 if a>b then return 1 end
 return 0
end
local function can_advance(session,wanted,enqueue)
 if not valid_generation(wanted) then return false end
 local current=redis.call('HGET',KEYS[2],session)
 if current and (not valid_generation(current) or generation_compare(current,wanted)>0) then return false end
 local released=redis.call('HGET',KEYS[2],'release:'..session)
 if released and (not valid_generation(released) or generation_compare(released,wanted)>0 or (enqueue and generation_compare(released,wanted)==0)) then return false end
 return true
end
local function has_type(key,wanted)
 local actual=redis.call('TYPE',key)
 if type(actual)=='table' then actual=actual.ok end
 return actual=='none' or actual==wanted
end
local function projection_types_ok()
 if not has_type(KEYS[1],'zset') or not has_type(KEYS[2],'hash') then return false end
 for i=3,#KEYS do if not has_type(KEYS[i],'string') then return false end end
 return true
end
`
