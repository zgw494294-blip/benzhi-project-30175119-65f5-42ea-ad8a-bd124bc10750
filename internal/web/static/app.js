const state={case:null,views:null};
const statuses=['draft','awaiting_consent','consented','pending_review','revision','approved','published'];
const names={draft:'草拟',awaiting_consent:'待同意',consented:'已同意',pending_review:'待复核',revision:'整改中',approved:'已批准',published:'已发布'};
const dispositionNames={pending:'未决',mask:'遮蔽',generalize:'泛化',keep:'保留',exclude:'排除片段'};
const $=selector=>document.querySelector(selector);
const uuid=()=>crypto.randomUUID();
function esc(value){const node=document.createElement('div');node.textContent=String(value??'');return node.innerHTML}
function escAttr(value){return esc(value).replaceAll('"','&quot;')}
function meta(){return{expectedVersion:state.case.case.version,idempotencyKey:uuid(),actor:'WEB-OPERATOR'}}
async function api(path,options={}){
  const response=await fetch(path,{headers:{'Content-Type':'application/json'},...options});
  const data=await response.json();
  if(!response.ok){const error=new Error(data.error?.message||'请求失败');error.details=data.error||{};throw error}
  return data;
}
function notice(message,error=false){const node=$('#notice');node.hidden=false;node.textContent=message;node.classList.toggle('error',error);setTimeout(()=>node.hidden=true,6000)}
function clearFieldErrors(){document.querySelectorAll('.field-errors').forEach(node=>node.textContent='')}
function showIssues(issues=[]){
  for(const issue of issues){
    if(issue.row){const node=document.querySelector('.batch-row:nth-child('+issue.row+') .field-errors');if(node)node.textContent+=(node.textContent?'；':'')+issue.field+'：'+issue.reason}
    if(issue.itemId){const node=document.querySelector('[data-item-error="'+CSS.escape(issue.itemId)+'"]');if(node)node.textContent+=(node.textContent?'；':'')+issue.field+'：'+issue.reason}
  }
}
async function refreshViews(){if(state.case)state.views=await api('/api/cases/'+state.case.case.id+'/views')}
async function run(task,success){
  clearFieldErrors();
  try{const value=await task();if(value?.case){state.case=value;await refreshViews();render()}if(success)notice(success);return value}
  catch(error){const issues=error.details?.issues||[];showIssues(issues);const summary=issues.map(issue=>(issue.itemId||(issue.row?'第 '+issue.row+' 行':issue.field))+'：'+issue.reason).join('；');notice(error.message+(summary?'（'+summary+'）':''),true);return null}
}
function body(form){return Object.fromEntries(new FormData(form).entries())}

function render(){
  const aggregate=state.case;if(!aggregate)return;
  localStorage.setItem('dialect-case',aggregate.case.id);
  $('#case-badge').textContent=aggregate.case.id.slice(0,8)+' · '+names[aggregate.case.status]+' · v'+aggregate.case.version;
  const current=statuses.indexOf(aggregate.case.status);
  $('#steps').innerHTML=statuses.map((status,index)=>'<div class="step '+(index<=current?'active':'')+'">'+(index+1)+' '+names[status]+'</div>').join('');
  document.querySelectorAll('.gated').forEach(node=>node.classList.remove('disabled'));
  $('#create-panel').style.display='none';
  const metadata=$('#metadata-form');
  metadata.contributorCode.value=aggregate.case.contributorCode;
  metadata.collectionContext.value=aggregate.case.collectionContext;
  metadata.languageTags.value=aggregate.case.languageTags.join(',');
  metadata.intendedAudience.value=aggregate.case.intendedAudience;
  renderSegments(aggregate);renderFindings(aggregate);renderScan(aggregate);renderTasks(aggregate);renderReviews(aggregate);renderViews();
  $('#credential').textContent=aggregate.credential?JSON.stringify(aggregate.credential,null,2):'等待签发';
  applyStatusControls(aggregate.case.status);
}

function renderSegments(aggregate){
  $('#segments').innerHTML=aggregate.segments.map((segment,index)=>
    '<div class="card segment-card"><input class="select-segment" type="checkbox" data-segment-select="'+segment.id+'" aria-label="选择片段">'
    +'<div><strong>#'+segment.sequence+' '+esc(segment.speakerCode)+'</strong> · '+esc(segment.category)+' · '+segment.startMillis+'–'+segment.endMillis+'ms · 修订 '+segment.revision+'<br>'+esc(segment.transcript)+'<br><small>'+esc(segment.id)+'</small></div>'
    +'<div class="segment-controls"><button type="button" class="ghost" data-move-segment="'+segment.id+'" data-delta="-1" '+(index===0?'disabled':'')+'>上移</button><button type="button" class="ghost" data-move-segment="'+segment.id+'" data-delta="1" '+(index===aggregate.segments.length-1?'disabled':'')+'>下移</button><button type="button" class="ghost" data-edit-segment="'+segment.id+'">编辑</button></div></div>'
  ).join('')||'<p>尚无片段</p>';
  document.querySelectorAll('[data-edit-segment]').forEach(button=>button.onclick=()=>beginSegmentEdit(button.dataset.editSegment));
  document.querySelectorAll('[data-move-segment]').forEach(button=>button.onclick=()=>moveSegment(button.dataset.moveSegment,Number(button.dataset.delta)));
}

function renderFindings(aggregate){
  const latestReturned=aggregate.reviews.filter(review=>review.decision==='returned').at(-1);
  const targets=new Set(latestReturned?.targetFindingIds||[]);
  $('#findings').innerHTML=aggregate.findings.map(finding=>{
    const enabled=aggregate.case.status==='consented'||(aggregate.case.status==='revision'&&targets.has(finding.id));
    const selected=finding.disposition==='pending'?'mask':finding.disposition;
    const options=['mask','generalize','keep','exclude'].map(value=>'<option value="'+value+'" '+(selected===value?'selected':'')+'>'+dispositionNames[value]+'</option>').join('');
    return '<div class="card finding"><input class="select-finding" type="checkbox" data-finding-select="'+finding.id+'" '+(enabled?'':'disabled')+'>'
      +'<div><strong>'+esc(finding.findingType)+' · '+esc(dispositionNames[finding.disposition])+'</strong><br><code>'+esc(finding.evidence)+'</code><br><small>'+esc(finding.id)+'</small><div class="field-errors" data-item-error="'+finding.id+'"></div></div>'
      +'<label>处置<select data-disp="'+finding.id+'" '+(enabled?'':'disabled')+'>'+options+'</select></label>'
      +'<label>理由<input data-reason="'+finding.id+'" value="'+escAttr(finding.rationale||'敏感信息最小化处理')+'" '+(enabled?'':'disabled')+'></label>'
      +'<button type="button" data-resolve="'+finding.id+'" '+(enabled?'':'disabled')+'>'+(finding.disposition==='pending'?'保存处置':'更新处置')+'</button></div>';
  }).join('')||'<p>尚未发现敏感候选；执行扫描后可提交复核。</p>';
  document.querySelectorAll('[data-resolve]').forEach(button=>button.onclick=()=>resolveOne(button.dataset.resolve));
}

function renderScan(aggregate){
  const scan=aggregate.scanHistory?.at(-1);
  if(!scan){$('#scan-summary').textContent='尚无扫描记录';return}
  const removed=scan.removedFindings?.map(finding=>finding.id).join('、')||'无';
  $('#scan-summary').innerHTML='最近规则 <strong>'+esc(scan.ruleVersion)+'</strong> · '+new Date(scan.executedAt).toLocaleString()+' · 新增 '+scan.addedCount+' / 未变化 '+scan.unchangedCount+' / 消失 '+scan.removedCount+'<br>集合摘要 <code>'+esc(scan.findingSetDigest)+'</code><br>消失候选（仅保留在差异记录）：'+esc(removed);
}

function renderTasks(aggregate){
  if(!aggregate.revisionTasks?.length){$('#revision-tasks').innerHTML='';return}
  $('#revision-tasks').innerHTML='<div class="task-list"><h3>定向整改任务轨迹</h3>'+aggregate.revisionTasks.map(task=>
    '<div class="card task '+(task.completed?'done':'pending')+'"><strong>第 '+task.roundNumber+' 轮 · '+esc(task.findingId)+' · '+(task.completed?'已完成':'未完成')+'</strong><br>复核意见：'+esc(task.reviewComment)+'<br>退回前：'+esc(dispositionNames[task.beforeDisposition])+' / '+esc(task.beforeRationale)+'<br>'
    +(task.afterDisposition?'整改后：'+esc(dispositionNames[task.afterDisposition])+' / '+esc(task.afterRationale):esc(task.incompleteReason))
    +(task.resubmittedToRound?'<br>已冻结并重提至第 '+task.resubmittedToRound+' 轮':'')+'</div>'
  ).join('')+'</div>';
}

function renderReviews(aggregate){
  $('#reviews').innerHTML=aggregate.reviews.map(review=>'<div class="card"><strong>第 '+review.roundNumber+' 轮 · '+esc(review.decision)+'</strong><br>提交摘要 <code>'+review.submissionDigest.slice(0,18)+'…</code><br>'+esc(review.comments||'等待复核')+(review.targetFindingIds?.length?'<br>退回目标：'+review.targetFindingIds.map(esc).join('、'):'')+'</div>').join('');
}

function renderViews(){
  if(!state.views)return;
  const checklist=state.views.checklist;
  $('#scope-digest').textContent=checklist.scopeDigest||'等待生成';
  $('#consent-form').scopeDigest.value=checklist.scopeDigest||'';
  $('#consent-scope').innerHTML=checklist.segments.map(item=>'<div class="card"><strong>#'+item.sequence+' · '+esc(item.segmentId)+'</strong><br>'+esc(item.speakerCode)+' · '+item.startMillis+'–'+item.endMillis+'ms · '+esc(item.category)+'<br>'+esc(item.transcriptSummary)+'</div>').join('')||'<p>尚无授权范围</p>';
  const receipt=$('#consent-receipt');receipt.hidden=!checklist.receipt;if(checklist.receipt)receipt.textContent=JSON.stringify(checklist.receipt,null,2);
  updatePurposeImpact();
}

function updatePurposeImpact(){
  const form=$('#consent-form');
  const values=[['研究用途',form.researchAllowed.checked,'不得用于研究'],['教学用途',form.teachingAllowed.checked,'不得用于教学'],['公开展示',form.publicDisplayAllowed.checked,'将阻断公开清单与发布凭据']];
  const node=$('#purpose-impact');node.classList.toggle('blocking',!form.publicDisplayAllowed.checked);
  node.innerHTML=values.map(value=>value[0]+'：<strong>'+(value[1]?'允许':'拒绝')+'</strong>，'+(value[1]?'可进入该用途后续流程':value[2])).join('<br>');
}

function applyStatusControls(status){
  const draft=status==='draft';
  $('#segment-form').querySelectorAll('input,textarea,button').forEach(node=>node.disabled=!draft);
  $('#metadata-form').querySelectorAll('input,textarea,button').forEach(node=>node.disabled=!draft);
  $('#batch-segment-box').querySelectorAll('input,textarea,button').forEach(node=>node.disabled=!draft);
  $('#revoke-segments').disabled=!draft;
  document.querySelectorAll('[data-edit-segment],[data-segment-select]').forEach(node=>node.disabled=!draft);
  if(!draft)document.querySelectorAll('[data-move-segment]').forEach(node=>node.disabled=true);
  $('#consent-form').querySelectorAll('input,button').forEach(node=>node.disabled=status!=='awaiting_consent');
  $('#scan').disabled=status!=='consented';
  $('#submit-review').disabled=!(status==='consented'||status==='revision');
  $('#review-form').querySelectorAll('input,textarea,select,button').forEach(node=>node.disabled=status!=='pending_review');
  $('#release').disabled=status!=='approved';
  $('#apply-bulk').disabled=!(status==='consented'||status==='revision');
  $('#save-selected').disabled=!(status==='consented'||status==='revision');
}

function addBatchRow(values={}){
  const rows=[...document.querySelectorAll('.batch-row')];
  const maximum=state.case?.segments?.reduce((value,item)=>Math.max(value,item.sequence),0)||0;
  const sequence=values.sequence??maximum+rows.length+1;
  const node=document.createElement('div');node.className='batch-row';
  node.innerHTML='<label>顺序号<input data-field="sequence" type="number" min="1" value="'+sequence+'"></label><label>说话者<input data-field="speakerCode" value="'+escAttr(values.speakerCode||'SPK-A')+'"></label><label>类别<input data-field="category" value="'+escAttr(values.category||'叙事')+'"></label><label>开始毫秒<input data-field="startMillis" type="number" min="0" value="'+(values.startMillis??0)+'"></label><label>结束毫秒<input data-field="endMillis" type="number" min="1" value="'+(values.endMillis??1000)+'"></label><label>文字转写<textarea data-field="transcript">'+esc(values.transcript||'')+'</textarea></label><button type="button" class="ghost remove-batch-row">移除</button><div class="field-errors"></div>';
  node.querySelector('.remove-batch-row').onclick=()=>node.remove();$('#batch-segment-rows').append(node);
}
function collectBatchRows(){return [...document.querySelectorAll('.batch-row')].map(row=>({sequence:Number(row.querySelector('[data-field="sequence"]').value),speakerCode:row.querySelector('[data-field="speakerCode"]').value,category:row.querySelector('[data-field="category"]').value,startMillis:Number(row.querySelector('[data-field="startMillis"]').value),endMillis:Number(row.querySelector('[data-field="endMillis"]').value),transcript:row.querySelector('[data-field="transcript"]').value}))}

$('#create-form').onsubmit=event=>{event.preventDefault();const input=body(event.target);input.languageTags=input.languageTags.split(',').map(value=>value.trim()).filter(Boolean);input.actor='ARCHIVIST-01';input.idempotencyKey=uuid();run(()=>api('/api/cases',{method:'POST',body:JSON.stringify(input)}),'案件已创建')};
$('#metadata-form').onsubmit=event=>{event.preventDefault();const input={...body(event.target),...meta()};input.languageTags=input.languageTags.split(',').map(value=>value.trim()).filter(Boolean);run(()=>api('/api/cases/'+state.case.case.id,{method:'PATCH',body:JSON.stringify(input)}),'案件信息已更新')};
$('#segment-form').onsubmit=async event=>{event.preventDefault();const input={...body(event.target),...meta()},id=input.segmentId;delete input.segmentId;['sequence','startMillis','endMillis'].forEach(key=>input[key]=Number(input[key]));const path=id?'/api/cases/'+state.case.case.id+'/segments/'+id:'/api/cases/'+state.case.case.id+'/segments';const result=await run(()=>api(path,{method:id?'PUT':'POST',body:JSON.stringify(input)}),id?'片段已更新':'片段已加入');if(result)resetSegmentForm()};
function beginSegmentEdit(id){const segment=state.case.segments.find(item=>item.id===id),form=$('#segment-form');for(const key of ['segmentId','sequence','speakerCode','startMillis','endMillis','transcript','category'])form[key].value=key==='segmentId'?segment.id:segment[key];$('#save-segment').textContent='保存片段修改';$('#cancel-edit').hidden=false;form.scrollIntoView({behavior:'smooth'})}
function resetSegmentForm(){const form=$('#segment-form');form.segmentId.value='';$('#save-segment').textContent='加入片段';$('#cancel-edit').hidden=true}
$('#cancel-edit').onclick=resetSegmentForm;
$('#add-batch-row').onclick=()=>addBatchRow();
$('#batch-segment-form').onsubmit=async event=>{event.preventDefault();const result=await run(()=>api('/api/cases/'+state.case.case.id+'/segments/batch',{method:'POST',body:JSON.stringify({...meta(),segments:collectBatchRows()})}),'整批片段已原子保存');if(result){$('#batch-segment-rows').innerHTML='';addBatchRow();addBatchRow();addBatchRow()}};
$('#revoke-segments').onclick=()=>{const segmentIds=[...document.querySelectorAll('[data-segment-select]:checked')].map(node=>node.dataset.segmentSelect);if(!confirm('确认撤销 '+segmentIds.length+' 条片段？'))return;run(()=>api('/api/cases/'+state.case.case.id+'/segments/revoke',{method:'POST',body:JSON.stringify({...meta(),segmentIds})}),'所选片段已撤销')};
function moveSegment(id,delta){const ids=state.case.segments.map(item=>item.id),index=ids.indexOf(id),target=index+delta;if(target<0||target>=ids.length)return;[ids[index],ids[target]]=[ids[target],ids[index]];run(()=>api('/api/cases/'+state.case.case.id+'/segments/reorder',{method:'POST',body:JSON.stringify({...meta(),segmentIds:ids})}),'片段顺序已重排')}
$('#request-consent').onclick=()=>run(()=>api('/api/cases/'+state.case.case.id+'/request-consent',{method:'POST',body:JSON.stringify(meta())}),'同意清单已生成');
$('#consent-form').querySelectorAll('input[type="checkbox"]').forEach(node=>node.onchange=updatePurposeImpact);
$('#consent-form').onsubmit=event=>{event.preventDefault();const form=event.target,input={...meta(),scopeDigest:form.scopeDigest.value,confirmedBy:form.confirmedBy.value,researchAllowed:form.researchAllowed.checked,teachingAllowed:form.teachingAllowed.checked,publicDisplayAllowed:form.publicDisplayAllowed.checked};run(()=>api('/api/cases/'+state.case.case.id+'/confirm-consent',{method:'POST',body:JSON.stringify(input)}),'同意基线与冻结回执已保存')};
$('#scan').onclick=()=>run(()=>api('/api/cases/'+state.case.case.id+'/scan',{method:'POST',body:JSON.stringify(meta())}),'敏感扫描完成，差异与继承处置已保存');
$('#preview').onclick=async()=>{const value=await run(()=>api('/api/cases/'+state.case.case.id+'/views'));if(value)$('#publication-preview').textContent=JSON.stringify(value.assessment,null,2)};
function resolveOne(id){const input={...meta(),disposition:document.querySelector('[data-disp="'+id+'"]').value,rationale:document.querySelector('[data-reason="'+id+'"]').value};run(()=>api('/api/cases/'+state.case.case.id+'/findings/'+id+'/resolve',{method:'POST',body:JSON.stringify(input)}),'处置已保存')}
function selectedDecisions(uniform=false){return [...document.querySelectorAll('[data-finding-select]:checked')].map(node=>{const id=node.dataset.findingSelect;return{findingId:id,disposition:uniform?$('#bulk-disposition').value:document.querySelector('[data-disp="'+id+'"]').value,rationale:uniform?$('#bulk-rationale').value:document.querySelector('[data-reason="'+id+'"]').value}})}
function saveDecisionBatch(uniform){const decisions=selectedDecisions(uniform);run(()=>api('/api/cases/'+state.case.case.id+'/findings/batch',{method:'POST',body:JSON.stringify({...meta(),decisions})}),'批量处置已原子保存')}
$('#apply-bulk').onclick=()=>saveDecisionBatch(true);$('#save-selected').onclick=()=>saveDecisionBatch(false);
$('#submit-review').onclick=()=>run(()=>api('/api/cases/'+state.case.case.id+'/submit-review',{method:'POST',body:JSON.stringify(meta())}),'已提交新的伦理复核轮次');
$('#review-form').onsubmit=event=>{event.preventDefault();const input={...body(event.target),...meta()};input.targetFindingIds=input.targetFindingIds.split(',').map(value=>value.trim()).filter(Boolean);run(()=>api('/api/cases/'+state.case.case.id+'/review',{method:'POST',body:JSON.stringify(input)}),'复核结论与整改快照已保存')};
$('#release').onclick=()=>run(()=>api('/api/cases/'+state.case.case.id+'/release',{method:'POST',body:JSON.stringify(meta())}),'发布凭据已签发');
$('#verify').onclick=async()=>{const value=await run(()=>api('/api/cases/'+state.case.case.id+'/verify'));if(value){$('#credential').textContent=JSON.stringify(value,null,2);notice(value.message,!value.valid)}};
$('#timeline').onclick=async()=>{const value=await run(()=>api('/api/cases/'+state.case.case.id+'/audit'));if(value)$('#audit').innerHTML=value.events.map(event=>'<div class="event"><strong>'+esc(event.action)+' · v'+event.beforeVersion+'→v'+event.afterVersion+'</strong><small>'+esc(event.actor)+' · '+new Date(event.occurredAt).toLocaleString()+'</small><br>'+esc(event.details)+'</div>').join('')};
$('#use-current-credential').onclick=()=>{$('#credential-input').value=state.case?.credential?JSON.stringify(state.case.credential,null,2):''};
$('#verify-presented').onclick=async()=>{let credential;try{credential=JSON.parse($('#credential-input').value)}catch(error){notice('粘贴内容不是有效 JSON',true);return}const value=await run(()=>api('/api/credentials/verify',{method:'POST',body:JSON.stringify(credential)}));if(!value)return;$('#verification-result').innerHTML='<div class="impact '+(value.valid?'':'blocking')+'"><strong>'+esc(value.message)+'</strong><br>冻结清单 '+value.manifestCount+' 条 · 批准轮次 '+value.approvalRound+' · 关联审计 '+value.relatedAuditEvents.length+' 条</div><div class="verification-components">'+value.components.map(component=>'<div class="component '+(component.consistent?'ok':'')+'"><strong>'+esc(component.label)+'：'+(component.consistent?'一致':'异常')+'</strong><br>'+esc(component.message)+'<code>凭据值：'+esc(component.credentialValue)+'<br>重算值：'+esc(component.recomputedValue)+'</code></div>').join('')+'</div>'};

addBatchRow();addBatchRow();addBatchRow();
const saved=localStorage.getItem('dialect-case');
if(saved){api('/api/cases/'+saved).then(async value=>{state.case=value;await refreshViews();render()}).catch(()=>localStorage.removeItem('dialect-case'))}
if(!saved){$('#steps').innerHTML=statuses.map((status,index)=>'<div class="step '+(index===0?'active':'')+'">'+(index+1)+' '+names[status]+'</div>').join('');document.querySelectorAll('.gated').forEach(node=>node.classList.add('disabled'))}
