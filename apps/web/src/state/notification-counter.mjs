import {errorMessage} from '../v3/domain-presentation.mjs';

// One Owner-wide projection. Character selection does not own this read.
export function createNotificationCounter({read,changed}) {
  let owner=null,session=0,epoch=0,active=true;
  const owns=(expected,generation)=>active&&owner===expected&&session===generation;
  async function refresh() {
    if(!active||!owner)return null;
    const expected=owner,generation=session,request=++epoch;
    changed({notificationCountLoading:true,notificationCountError:''});
    try {
      const value=await read();
      if(!owns(expected,generation)||request!==epoch)return null;
      if(!value||!Number.isSafeInteger(value.count)||value.count<0||
        (value.alert_count!==undefined&&(!Number.isSafeInteger(value.alert_count)||value.alert_count<0||value.alert_count>value.count)))
        throw Error('未读计数尚未完整读到。');
      changed({notificationCount:{...value},notificationCountLoading:false,notificationCountError:''});
      return value;
    } catch(error) {
      if(owns(expected,generation)&&request===epoch)
        changed({notificationCountLoading:false,notificationCountError:errorMessage(error)});
      return null;
    }
  }
  return {
    refresh,
    reset(next) {
      if(!active)return;
      owner=next||null;session++;epoch++;
      changed({notificationCount:null,notificationCountLoading:false,notificationCountError:''});
    },
    async afterWrite(execute) {
      if(!active||!owner)throw Error('请先登录后核对通知。');
      const expected=owner,generation=session,result=await execute();
      if(owns(expected,generation)) {
        // Invalidate every count read begun before the write was confirmed.
        epoch++;
        void refresh();
      }
      return result;
    },
    dispose(){active=false;session++;epoch++;},
  };
}
