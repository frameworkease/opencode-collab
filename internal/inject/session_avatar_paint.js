/* session_avatar_paint.js — DOM painter; avatars use same-origin URL + browser HTTP cache */
function findAvatarNode(row){
  if(!row)return null;
  var slotted=row.querySelector("[data-opencode-collab-avatar-slot]");
  if(slotted)return slotted;
  var applied=row.querySelector("[data-opencode-collab-avatar-applied]");
  if(applied)return applied;
  return null;
}
/* OpenCode user bubbles have no default avatar disc — inject a local slot. */
function ensureAvatarNode(row){
  if(!row||row.getAttribute("data-opencode-collab-dup-user"))return null;
  var um=row.querySelector("[data-component=\"user-message\"]");
  if(!um)return null;
  /* Prefer text body; attachment-only → attachments. Never full-width user-message. */
  var host=um.querySelector("[data-slot=\"user-message-body\"]")||
    um.querySelector("[data-slot=\"user-message-attachments\"]");
  if(!host)return null;
  var existing=findAvatarNode(row);
  if(existing){
    try{if(existing.parentElement!==host)host.appendChild(existing);}catch(e){}
    return existing;
  }
  var el=document.createElement("div");
  el.className="h-8 w-8 flex-shrink-0 rounded-full";
  el.setAttribute("data-opencode-collab-avatar-slot","1");
  el.setAttribute("aria-hidden","true");
  host.appendChild(el);
  return el;
}
function cssUrl(src){
  return "url(\""+String(src||"").replace(/\\/g,"\\\\").replace(/"/g,"\\\"")+"\")";
}
function docIsDark(){
  try{
    var root=document.documentElement;
    if(root&&root.getAttribute("data-color-scheme")==="dark")return true;
    if(root&&root.classList&&root.classList.contains("dark"))return true;
  }catch(e){}
  return false;
}
function initialBg(attr){
  var ch=initialOf(attr)||"?";
  var dark=docIsDark();
  var fill=dark?"#f5f5f5":"#1b1b1b";
  var bg=dark?"#141414":"#f7f7f7";
  var svg="<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"64\" height=\"64\">"+
    "<rect width=\"64\" height=\"64\" fill=\""+bg+"\"/>"+
    "<text x=\"50%\" y=\"54%\" text-anchor=\"middle\" dominant-baseline=\"middle\" "+
    "font-size=\"22\" font-weight=\"700\" fill=\""+fill+"\" "+
    "font-family=\"system-ui,-apple-system,sans-serif\">"+String(ch).replace(/[<&]/g,"")+"</text></svg>";
  return cssUrl("data:image/svg+xml,"+encodeURIComponent(svg));
}
function avatarHandle(a){
  var u=String((a&&a.username)||"").trim();
  if(u.charAt(0)==="@")u=u.slice(1);
  return u;
}
function avatarTipLabel(a){
  var name=String((a&&(a.nickname||a.username))||"").trim();
  var handle=avatarHandle(a);
  if(name&&handle)return name+"\n"+handle;
  return name||handle||"";
}
var avatarTipEl=null;
var avatarTipHideTimer=0;
function hideAvatarTip(){
  if(avatarTipHideTimer){clearTimeout(avatarTipHideTimer);avatarTipHideTimer=0;}
  if(avatarTipEl&&avatarTipEl.parentNode)avatarTipEl.parentNode.removeChild(avatarTipEl);
  avatarTipEl=null;
}
function placeAvatarTip(anchor){
  if(!avatarTipEl||!anchor)return;
  var r=anchor.getBoundingClientRect();
  var tip=avatarTipEl.getBoundingClientRect();
  var x=r.left+r.width/2-tip.width/2;
  var y=r.top-tip.height-8;
  if(y<8)y=r.bottom+8;
  if(x<8)x=8;
  if(x+tip.width>window.innerWidth-8)x=Math.max(8,window.innerWidth-tip.width-8);
  avatarTipEl.style.left=Math.round(x)+"px";
  avatarTipEl.style.top=Math.round(y)+"px";
}
function showAvatarTip(anchor, attr){
  hideAvatarTip();
  var name=String((attr&&(attr.nickname||attr.username))||"").trim();
  var handle=avatarHandle(attr);
  if(!name&&!handle)return;
  var tip=document.createElement("div");
  tip.className="opencode-collab-avatar-tip";
  tip.setAttribute("role","tooltip");
  if(name){
    var n=document.createElement("span");
    n.className="opencode-collab-avatar-tip__name";
    n.textContent=name;
    tip.appendChild(n);
  }
  if(handle && handle.toLowerCase()!==String(name||"").replace(/^@/,"").toLowerCase()){
    var u=document.createElement("span");
    u.className="opencode-collab-avatar-tip__user";
    u.textContent=handle;
    tip.appendChild(u);
  }else if(!name && handle){
    var only=document.createElement("span");
    only.className="opencode-collab-avatar-tip__user";
    only.textContent=handle;
    tip.appendChild(only);
  }
  document.body.appendChild(tip);
  avatarTipEl=tip;
  placeAvatarTip(anchor);
}
function bindAvatarHover(el, attr){
  if(!el)return;
  el.__userAvatarAttr=attr;
  if(el.getAttribute("data-opencode-collab-avatar-tipbound"))return;
  el.setAttribute("data-opencode-collab-avatar-tipbound","1");
  el.addEventListener("mouseenter",function(){
    showAvatarTip(el, el.__userAvatarAttr||attr);
  });
  el.addEventListener("mouseleave",function(){
    avatarTipHideTimer=setTimeout(hideAvatarTip, 60);
  });
  el.addEventListener("focus",function(){showAvatarTip(el, el.__userAvatarAttr||attr);});
  el.addEventListener("blur",hideAvatarTip);
}
function avatarPaintSame(el, attr){
  if(!el||!attr)return false;
  var id=String(attr.userId||"1");
  if((el.getAttribute("data-opencode-collab-avatar-applied")||"")!==id)return false;
  var name=String((attr.nickname||attr.username)||"").trim();
  if((el.getAttribute("data-opencode-collab-name")||"")!==name)return false;
  var bg=(el.style&&el.style.backgroundImage)||"";
  var want=String(attr.avatarUrl||"");
  if(want)return bg===cssUrl(want)||bg.indexOf(want)>=0;
  return bg.indexOf("data:image/svg+xml")>=0;
}
function applyAvatar(el, attr){
  if(!el||!attr)return;
  applyAvatarNow(el, attr);
  var uid=String(attr.userId||"").trim();
  if(!uid)return;
  ensureProfile(uid, function(p){
    if(!p)return;
    var merged={
      userId:uid,
      username:String(p.username||""),
      nickname:String(p.nickname||attr.nickname||""),
      avatarUrl:String(p.avatarUrl||""),
      messageId:attr.messageId,
      timestamp:attr.timestamp
    };
    /* Skip identical re-paint — Safari flashes when background-image is reassigned. */
    if(avatarPaintSame(el, merged))return;
    applyAvatarNow(el, merged);
  });
}
function revealAvatar(el){
  if(!el)return;
  try{el.setAttribute("data-opencode-collab-avatar-ready","1");}catch(e){}
}
function applyAvatarNow(el, attr){
  if(!el||!attr)return;
  var id=attr.userId||"1";
  var tip=avatarTipLabel(attr);
  var name=String((attr.nickname||attr.username)||"").trim();
  /* Attributes + background only — never textContent/appendChild/reparent. */
  el.setAttribute("data-opencode-collab-avatar-applied", id);
  if(name)el.setAttribute("data-opencode-collab-name", name);
  else el.removeAttribute("data-opencode-collab-name");
  el.removeAttribute("title");
  if(tip)el.setAttribute("aria-label", tip.replace(/\n/g," "));
  bindAvatarHover(el, attr);
  var curBg=(el.style&&el.style.backgroundImage)||"";
  if(attr.avatarUrl){
    var url=String(attr.avatarUrl);
    var next=cssUrl(url);
    if(curBg===next||curBg.indexOf(url)>=0){
      revealAvatar(el);
      return;
    }
    try{el.removeAttribute("data-opencode-collab-avatar-ready");}catch(e){}
    el.style.setProperty("background-image", next, "important");
    var img=new Image();
    var done=false;
    function finish(ok){
      if(done)return;
      done=true;
      if(el.getAttribute("data-opencode-collab-avatar-applied")!==id)return;
      if(!ok)el.style.setProperty("background-image", initialBg(attr), "important");
      revealAvatar(el);
    }
    img.onload=function(){finish(true);};
    img.onerror=function(){finish(false);};
    img.src=url;
    if(img.complete&&img.naturalWidth)finish(true);
  }else{
    var init=initialBg(attr);
    if(curBg===init){
      revealAvatar(el);
      return;
    }
    try{el.removeAttribute("data-opencode-collab-avatar-ready");}catch(e){}
    el.style.setProperty("background-image", init, "important");
    revealAvatar(el);
  }
}

function schedulePaint(){
  if(paintDepth)return;
  if(paintTimer)clearTimeout(paintTimer);
  paintTimer=setTimeout(function(){ paintAll(); }, 30);
}
function paintAll(){
  paintDepth++;
  try{
  try{syncSessionFromLocation();}catch(e){}
  var sid=sessionIdFromLocation()||currentSession;
  var rows=userMessageRows();
  if(!rows.length)return;
  try{dedupeUserBubbles();}catch(e){}
  for(var i=0;i<rows.length;i++){
    (function(row){
      if(row.getAttribute("data-opencode-collab-dup-user"))return;
      var rawText=bubbleText(row);
      if(isSyntheticUserBubble(rawText)){
        scrubSyntheticUserAttribution(row);
        return;
      }
      var av=ensureAvatarNode(row);
      if(!av)return;
      var appliedId=av.getAttribute("data-opencode-collab-avatar-applied")||"";
      var mid=rowMessageId(row);
      var norm=normalizeContent(rawText);
      var attr=pickAttrForPaint(sid, mid);
      /* Local pending seed; peer live comes from SSE inbox.enqueued → byMessageId. */
      if(!attr&&mid&&sid){
        try{
          if(seedOptimisticLocalAttr(sid, mid, norm))attr=pickAttrByMessageId(sid, mid);
        }catch(e){}
      }
      if(!attr)return;
      attr=hydrateAttrFromProfile(attr);
      var wantId=String(attr.userId||"");
      var wantAv=String(attr.avatarUrl||"");
      var hasBg=(av.style&&av.style.backgroundImage)||"";
      var missingPhoto=!!wantAv&&(!hasBg||hasBg.indexOf("data:image/svg+xml")>=0);
      if(appliedId&&appliedId===wantId&&!missingPhoto){
        var nm=String((attr.nickname||attr.username)||"").trim();
        if(nm&&(av.getAttribute("data-opencode-collab-name")||"")!==nm)applyAvatar(av, attr);
        return;
      }
      applyAvatar(av, attr);
    })(rows[i]);
  }
  }finally{
    paintDepth--;
  }
}
