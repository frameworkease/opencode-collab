/* Must run in <head> before OpenCode index-*.js.
   OpenCode 2.x reads /api/event via fetch+ReadableStream (not the EventSource API).
   Tee the SSE body and forward session.inbox.enqueued to the late handler.
   Drain eagerly and drop parse backlog so OpenCode's tee half is not stalled. */
(function(){
  if(window.__OPENCODE_COLLAB_SSE_TEE__)return;
  window.__OPENCODE_COLLAB_SSE_TEE__=1;
  window.__OPENCODE_COLLAB_INBOX_Q__=window.__OPENCODE_COLLAB_INBOX_Q__||[];
  window.__OPENCODE_COLLAB_ON_INBOX_ENQUEUED__=function(data){
    try{window.__OPENCODE_COLLAB_INBOX_Q__.push(data||{});}catch(e){}
    try{
      var q=window.__OPENCODE_COLLAB_INBOX_Q__;
      if(q&&q.length>64)q.splice(0, q.length-64);
    }catch(e){}
  };
  function emitInbox(data){
    try{
      var fn=window.__OPENCODE_COLLAB_ON_INBOX_ENQUEUED__;
      if(typeof fn==="function")fn(data||{});
    }catch(e){}
  }
  function handleSseBlock(block){
    if(!block)return;
    var lines=String(block).split("\n");
    var dataLines=[];
    for(var i=0;i<lines.length;i++){
      var line=lines[i];
      if(line.indexOf("data:")===0)dataLines.push(line.slice(5).replace(/^\s/, ""));
    }
    if(!dataLines.length)return;
    try{
      var j=JSON.parse(dataLines.join("\n"));
      if(j&&j.type==="session.inbox.enqueued")emitInbox(j.data||{});
    }catch(e){}
  }
  var PARSE_BUF_MAX=256*1024;
  function pumpSse(stream){
    if(!stream||!stream.getReader)return;
    var reader=stream.getReader();
    var dec=new TextDecoder();
    var buf="";
    var parseQ=[];
    var parsing=false;
    function flushParse(){
      if(parsing)return;
      parsing=true;
      function step(){
        try{
          while(parseQ.length){
            var chunk=parseQ.shift();
            if(chunk)buf+=chunk;
            if(buf.length>PARSE_BUF_MAX){
              /* Drop oldest unparsed text — keep draining so OpenCode half stays live. */
              buf=buf.slice(buf.length-Math.floor(PARSE_BUF_MAX/2));
            }
            buf=buf.replace(/\r\n/g,"\n").replace(/\r/g,"\n");
            var idx;
            while((idx=buf.indexOf("\n\n"))>=0){
              handleSseBlock(buf.slice(0, idx));
              buf=buf.slice(idx+2);
            }
          }
        }catch(e){}
        parsing=false;
        if(parseQ.length)flushParse();
      }
      try{queueMicrotask(step);}catch(e){setTimeout(step,0);}
    }
    function tick(){
      reader.read().then(function(next){
        try{
          if(next.value){
            var piece=dec.decode(next.value,{stream:!next.done});
            parseQ.push(piece);
            /* Cap queued raw chunks — prefer drop over stalling the OpenCode stream. */
            if(parseQ.length>32)parseQ.splice(0, parseQ.length-16);
            flushParse();
          }
          if(next.done){
            try{parseQ.push("\n\n");flushParse();}catch(e){}
            try{reader.releaseLock();}catch(e){}
            return;
          }
        }catch(e){}
        tick();
      }).catch(function(){
        try{reader.releaseLock();}catch(e){}
      });
    }
    tick();
  }
  if(typeof window.fetch!=="function")return;
  var origFetch=window.fetch;
  window.fetch=function(input, init){
    var url="";
    try{
      if(typeof input==="string")url=input;
      else if(input&&typeof input.url==="string")url=input.url;
    }catch(e){}
    var pathOnly=String(url).split("?")[0];
    var isEvent=/\/api\/event\/?$/i.test(pathOnly);
    var p=origFetch.apply(this, arguments);
    if(!isEvent)return p;
    return p.then(function(res){
      try{
        if(!res||!res.body||typeof res.body.tee!=="function")return res;
        var parts=res.body.tee();
        pumpSse(parts[0]);
        return new Response(parts[1],{status:res.status,statusText:res.statusText,headers:res.headers});
      }catch(e){
        return res;
      }
    });
  };
})();
