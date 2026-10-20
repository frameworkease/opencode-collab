/* session_avatar.js — collab avatars on OpenCode v2 timeline.
   Main path:
   1) sidecar POST …/prompt → metadata.sender
   2) noteUserMessageRecord → byMessageId[mid]
   3) head fetch tee /api/event session.inbox.enqueued → (2)
   4) paintAll: mid → hydrate(profileCache) → applyAvatar
   History: GET …/message list → (2). Local optimistic: mid-keyed seed only. */
var currentSession="";
var paintTimer=0;
var paintDepth=0; /* suppress MutationObserver re-entry while paintAll inserts slots */
var pendingLocal=null; /* {sid,norm,until} mid-keyed optimistic local send */
var pendingInbox=[]; /* inbox.enqueued while currentSession not set yet */
var messagesAPIBySession={}; /* sid -> {byMessageId:{mid:attr}} */

function setCurrentSession(sid){
  sid=String(sid||"").trim();
  if(!sid)return;
  currentSession=sid;
  flushPendingInbox();
}
function sessionIdFromLocation(){
  try{
    var m=String(location.pathname||"").match(/\/session\/([^/?#]+)/);
    return m?decodeURIComponent(m[1]):"";
  }catch(e){return "";}
}
function syncSessionFromLocation(){
  var sid=sessionIdFromLocation();
  if(sid)setCurrentSession(sid);
}
function flushPendingInbox(){
  if(!currentSession||!pendingInbox.length)return;
  var q=pendingInbox;
  pendingInbox=[];
  for(var i=0;i<q.length;i++)onInboxEnqueued(q[i]);
}

function apiUrl(path){return String(API||"").replace(/\/$/,"")+path;}
function profileURL(userId){
  return apiUrl("/profile/"+encodeURIComponent(String(userId||"")));
}
var profileCache={};
var profileInflight={};
var PROFILE_TTL_MS=10*60*1000;

function rememberProfile(p){
  if(!p)return;
  var id=String(p.userId||p.id||"").trim();
  if(!id)return;
  var prev=profileCache[id];
  var username=String(p.username||(prev&&prev.username)||"");
  var nickname=String(p.nickname||(prev&&prev.nickname)||"");
  var avatarUrl=String(p.avatarUrl||(prev&&prev.avatarUrl)||"");
  /* userId-only stubs must not stamp fetchedAt — otherwise ensureProfile
     treats the empty entry as fresh and never GETs. */
  var hasIdentity=!!(username||nickname||avatarUrl);
  var prevFetched=prev?Number(prev.fetchedAt||0):0;
  profileCache[id]={
    userId:id,
    username:username,
    nickname:nickname,
    avatarUrl:avatarUrl,
    fetchedAt:hasIdentity?Date.now():prevFetched
  };
  try{refreshUserPaintFromProfile(id);}catch(e){}
}

/* Profile updates only schedule paint — do not write back into byMessageId. */
function refreshUserPaintFromProfile(userId){
  if(!String(userId||"").trim())return;
  schedulePaint();
}

function hydrateAttrFromProfile(attr){
  if(!attr)return attr;
  var uid=String(attr.userId||"").trim();
  if(!uid)return attr;
  var cached=profileCache[uid];
  if(!cached)return attr;
  return {
    userId:uid,
    username:String(cached.username||""),
    nickname:String(cached.nickname||""),
    avatarUrl:String(cached.avatarUrl||""),
    messageId:attr.messageId,
    timestamp:attr.timestamp,
    norm:attr.norm
  };
}

function ensureProfile(userId, done){
  var id=String(userId||"").trim();
  if(!id){if(done)done(null);return;}
  var hit=profileCache[id];
  var fetchedAt=hit?Number(hit.fetchedAt||0):0;
  var complete=!!(hit&&fetchedAt>0&&(hit.avatarUrl||hit.username||hit.nickname));
  if(complete&&(Date.now()-fetchedAt)<PROFILE_TTL_MS){
    if(done)done(hit);
    return;
  }
  if(!profileInflight[id])profileInflight[id]=[];
  if(done)profileInflight[id].push(done);
  if(profileInflight[id].length>1)return;
  fetch(profileURL(id),{credentials:"same-origin",cache:"no-store"}).then(function(res){
    return res.ok?res.json():null;
  }).then(function(body){
    var u=body&&body.user?body.user:null;
    if(u){
      rememberProfile({
        id:u.id||id,
        username:u.username,
        nickname:u.nickname,
        avatarUrl:u.avatarUrl
      });
    }else if(hit){
      hit.fetchedAt=Date.now();
    }
    var cbs=profileInflight[id]||[];
    delete profileInflight[id];
    var out=profileCache[id]||null;
    for(var i=0;i<cbs.length;i++){try{cbs[i](out);}catch(e){}}
  }).catch(function(){
    var cbs=profileInflight[id]||[];
    delete profileInflight[id];
    for(var i=0;i<cbs.length;i++){try{cbs[i](profileCache[id]||null);}catch(e){}}
  });
}
try{if(ACTOR&&ACTOR.userId)rememberProfile(ACTOR);}catch(e){}

function normalizeContent(s){return String(s||"").replace(/\s+/g," ").trim();}
function messageTimeISO(m){
  if(!m)return "";
  if(m.timestamp!=null)return String(m.timestamp);
  if(m.createdAt!=null)return String(m.createdAt);
  var t=m.time;
  if(t!=null&&typeof t==="object"&&t.created!=null){
    var n=Number(t.created);
    if(Number.isFinite(n)){
      try{return new Date(n).toISOString();}catch(e){}
    }
    return String(t.created);
  }
  if(t!=null)return String(t);
  return "";
}

function userMessageSelector(){
  return "[data-timeline-row=\"UserMessage\"]";
}
function userMessageRows(){
  return document.querySelectorAll(userMessageSelector());
}
function rowMessageId(row){
  if(!row)return "";
  return String(row.getAttribute("data-message-id")||"").trim();
}
function initialOf(a){
  var n=(a&&(a.nickname||a.username))||"?";
  n=String(n).replace(/^@/,"").trim();
  if(!n)return "?";
  var parts=n.split(/\s+/);
  if(parts.length>=2)return (parts[0].charAt(0)+parts[1].charAt(0)).toUpperCase().slice(0,2);
  return n.slice(0,2).toUpperCase();
}
function bubbleText(row){
  if(!row)return "";
  var v2=row.querySelector("[data-slot=\"user-message-text\"]");
  if(v2)return v2.textContent||"";
  return "";
}
function isSyntheticUserBubble(s){
  s=String(s||"").replace(/^\s+/,"");
  if(!s)return false;
  if(s.indexOf("<system_notification>")===0)return true;
  if(s.indexOf("<system_reminder>")===0)return true;
  if(s.indexOf("<task>")===0)return true;
  if(/^Briefly inform the user\b/i.test(s))return true;
  return false;
}

function messageTextFromRecord(m){
  if(!m)return "";
  if(typeof m.content==="string")return m.content;
  if(typeof m.text==="string")return m.text;
  if(Array.isArray(m.content)){
    var out="";
    for(var ci=0;ci<m.content.length;ci++){
      var part=m.content[ci];
      if(part&&part.type==="text"&&typeof part.text==="string")out+=part.text;
    }
    return out;
  }
  return "";
}
function senderIdFromRecord(m){
  var meta=m&&m.metadata&&typeof m.metadata==="object"?m.metadata:null;
  if(!meta)return "";
  var v=meta.sender;
  if(v&&typeof v==="object")return String(v.userId||v.id||"").trim();
  return String(v||"").trim();
}

function ensureSessionSnap(sid){
  if(!messagesAPIBySession[sid])messagesAPIBySession[sid]={byMessageId:{}};
  if(!messagesAPIBySession[sid].byMessageId)messagesAPIBySession[sid].byMessageId={};
  return messagesAPIBySession[sid];
}

/* Attribution record: userId + mid (+ norm/timestamp). Display via hydrate. */
function putUserPaint(sid, entry){
  if(!entry||!entry.userId)return;
  var mid=String(entry.messageId||"").trim();
  if(!mid)return;
  var snap=ensureSessionSnap(sid);
  var prev=snap.byMessageId[mid]||{};
  snap.byMessageId[mid]={
    messageId:mid,
    userId:String(entry.userId),
    timestamp:entry.timestamp||prev.timestamp||"",
    norm:entry.norm||prev.norm||""
  };
}

function noteUserMessageRecord(sid, m){
  sid=String(sid||"");
  if(!sid||!m)return false;
  var role=String(m.role||m.type||"").toLowerCase();
  var kind=String(m.kind||"");
  if(role!=="user")return false;
  if(kind&&kind!=="text")return false;
  var mid=String(m.id||m.messageID||m.messageId||"").trim();
  var uid=senderIdFromRecord(m);
  var rawContent=messageTextFromRecord(m);
  var userNorm=normalizeContent(rawContent);
  if(!uid||!(userNorm||mid))return false;
  if(!mid)return false;
  putUserPaint(sid, {
    messageId:mid,
    norm:userNorm,
    timestamp:messageTimeISO(m),
    userId:uid
  });
  ensureProfile(uid, function(){schedulePaint();});
  schedulePaint();
  return true;
}

function seedOptimisticLocalAttr(sid, messageId, norm){
  if(!ACTOR||!ACTOR.userId||!sid||!messageId)return false;
  if(pickAttrByMessageId(sid, messageId))return true;
  if(!(pendingLocal&&pendingLocal.sid===sid&&Date.now()<pendingLocal.until))return false;
  /* Match by mid during pending window; norm is optional (bubble text may lag). */
  if(pendingLocal.norm&&norm&&pendingLocal.norm!==norm)return false;
  putUserPaint(sid, {
    messageId:messageId,
    norm:pendingLocal.norm||norm||"",
    timestamp:"",
    userId:ACTOR.userId
  });
  return true;
}

function noteMessagesAPI(sid, body){
  sid=String(sid||"");
  if(!sid||!body)return;
  var msgs;
  if(Array.isArray(body.data))msgs=body.data;
  else msgs=(body.data&&body.data.messages)||body.messages||[];
  if(!Array.isArray(msgs))return;
  for(var i=0;i<msgs.length;i++){
    noteUserMessageRecord(sid, msgs[i]||{});
  }
}

/* SSE session.inbox.enqueued carries full inbox item (incl. sender). */
function onInboxEnqueued(data){
  if(!data||typeof data!=="object")return;
  var sid=String(data.sessionID||"").trim();
  var mid=String(data.inboxID||"").trim();
  var item=data.item;
  if(!sid||!mid||!item||item.type!=="user")return;
  if(!currentSession){
    pendingInbox.push(data);
    if(pendingInbox.length>40)pendingInbox.shift();
    return;
  }
  if(sid!==currentSession)return;
  if(pickAttrByMessageId(sid, mid))return;
  var payload=item.payload&&typeof item.payload==="object"?item.payload:{};
  var meta=payload.metadata&&typeof payload.metadata==="object"?payload.metadata:{};
  var msg={
    id:mid,
    type:"user",
    text:typeof payload.text==="string"?payload.text:"",
    metadata:meta
  };
  if(!senderIdFromRecord(msg))return;
  noteUserMessageRecord(sid, msg);
}

function pickAttrByMessageId(sid, messageId){
  messageId=String(messageId||"").trim();
  if(!sid||!messageId)return null;
  var snap=messagesAPIBySession[sid];
  if(!snap||!snap.byMessageId)return null;
  var p=snap.byMessageId[messageId];
  return p?hydrateAttrFromProfile(p):null;
}
/* SPA tab switches often remount bubbles without refetching /message, so
   currentSession can still point at the previous tab. Prefer URL session, then
   fall back to any cached mid (message ids are globally unique). */
function pickAttrForPaint(sid, messageId){
  var attr=pickAttrByMessageId(sid, messageId);
  if(attr)return attr;
  messageId=String(messageId||"").trim();
  if(!messageId)return null;
  for(var other in messagesAPIBySession){
    if(!Object.prototype.hasOwnProperty.call(messagesAPIBySession, other))continue;
    if(other===sid)continue;
    attr=pickAttrByMessageId(other, messageId);
    if(attr)return attr;
  }
  return null;
}

function markDupUser(row){
  if(!row)return;
  row.setAttribute("data-opencode-collab-dup-user","1");
  row.style.setProperty("display","none","important");
}
function clearDupUser(row){
  if(!row)return;
  if(row.getAttribute("data-opencode-collab-dup-user"))row.removeAttribute("data-opencode-collab-dup-user");
  row.style.removeProperty("display");
}
function scrubSyntheticUserAttribution(row){
  if(!row||!isSyntheticUserBubble(bubbleText(row)))return;
  clearDupUser(row);
  var av=findAvatarNode(row);
  if(av){
    av.removeAttribute("data-opencode-collab-avatar-applied");
    av.removeAttribute("data-opencode-collab-name");
    av.style.removeProperty("background-image");
  }
}

/* Dedupe only by messageId — keep the first painted row if duplicates appear. */
function dedupeUserBubbles(){
  var rows=[].slice.call(userMessageRows());
  var seen={};
  for(var i=0;i<rows.length;i++){
    var row=rows[i];
    if(isSyntheticUserBubble(bubbleText(row))){
      scrubSyntheticUserAttribution(row);
      continue;
    }
    var mid=rowMessageId(row);
    if(!mid){clearDupUser(row);continue;}
    if(seen[mid]){markDupUser(row);continue;}
    seen[mid]=1;
    clearDupUser(row);
  }
}

(function(){
  if(!window.fetch)return;
  var orig=window.fetch;
  window.fetch=function(input, init){
    var url="";
    try{
      if(typeof input==="string")url=input;
      else if(input&&typeof input.url==="string")url=input.url;
    }catch(e){}
    var method="";
    try{
      method=String((init&&init.method)||(input&&input.method)||"GET").toUpperCase();
    }catch(e){method="GET";}
    var pathOnly=String(url).split("?")[0];
    var listMatch=pathOnly.match(/\/api\/session\/([^/]+)\/message\/?$/i);
    var oneMatch=pathOnly.match(/\/api\/session\/([^/]+)\/message\/([^/]+)/i);
    /* Only POST …/prompt gets metadata.sender from the sidecar. */
    var promptMatch=pathOnly.match(/\/api\/session\/([^/]+)\/prompt\/?$/i);
    if(method==="POST"&&promptMatch){
      var psid=decodeURIComponent(promptMatch[1]);
      setCurrentSession(psid);
      try{
        var rawBody=(init&&typeof init.body==="string")?init.body:"";
        var parsed=rawBody?JSON.parse(rawBody):null;
        var sendText=parsed&&typeof parsed.text==="string"?parsed.text:"";
        var sendNorm=normalizeContent(sendText);
        if(ACTOR&&ACTOR.userId&&sendNorm){
          pendingLocal={sid:psid, norm:sendNorm, until:Date.now()+8000};
        }
      }catch(e){}
      var pPrompt=orig.apply(this, arguments);
      pPrompt.then(function(res){
        try{
          res.clone().json().then(function(body){
            /* OpenCode returns Session.Inbox.User: {id,type,payload:{text,metadata},time}. */
            var inbox=body&&body.data?body.data:body;
            var payload=inbox&&inbox.payload?inbox.payload:{};
            var msg={
              id:inbox&&(inbox.id||inbox.messageID),
              type:"user",
              text:payload.text||(inbox&&inbox.text)||"",
              metadata:payload.metadata||(inbox&&inbox.metadata)||{},
              time:inbox&&inbox.time
            };
            try{noteUserMessageRecord(psid, msg);}catch(e){}
            pendingLocal=null;
          }).catch(function(){pendingLocal=null;});
        }catch(e){pendingLocal=null;}
        /* One short bump for late data-message-id; paintAll also dedupes. */
        setTimeout(function(){schedulePaint();}, 100);
      }).catch(function(){pendingLocal=null;});
      return pPrompt;
    }
    var p=orig.apply(this, arguments);
    if(listMatch&&!oneMatch){
      var sid=decodeURIComponent(listMatch[1]);
      setCurrentSession(sid);
      p.then(function(res){
        try{
          res.clone().json().then(function(body){
            try{noteMessagesAPI(sid, body);}catch(e){}
            schedulePaint();
          }).catch(function(){});
        }catch(e){}
        setTimeout(function(){schedulePaint();}, 100);
      }).catch(function(){});
    }
    return p;
  };
})();

/* Bind to <head> fetch-tee; drain events queued before this script. */
(function(){
  window.__OPENCODE_COLLAB_ON_INBOX_ENQUEUED__=onInboxEnqueued;
  var q=window.__OPENCODE_COLLAB_INBOX_Q__||[];
  window.__OPENCODE_COLLAB_INBOX_Q__=[];
  for(var i=0;i<q.length;i++)onInboxEnqueued(q[i]);
})();

(function(){
  function onRoute(){
    try{syncSessionFromLocation();}catch(e){}
    schedulePaint();
  }
  function boot(){
    onRoute();
    try{
      var obs=new MutationObserver(function(){schedulePaint();});
      obs.observe(document.documentElement,{childList:true,subtree:true});
    }catch(e){}
    try{
      if(!window.__OPENCODE_COLLAB_HISTORY_WRAP__){
        window.__OPENCODE_COLLAB_HISTORY_WRAP__=1;
        var wrap=function(fn){
          return function(){
            var ret=fn.apply(this, arguments);
            try{onRoute();}catch(e){}
            return ret;
          };
        };
        history.pushState=wrap(history.pushState.bind(history));
        history.replaceState=wrap(history.replaceState.bind(history));
        window.addEventListener("popstate", onRoute);
      }
    }catch(e){}
  }
  if(document.readyState==="loading")document.addEventListener("DOMContentLoaded",boot);
  else boot();
})();
