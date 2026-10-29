import {WorldArt,StoryMark} from "../shared/WorkspaceArtwork";
import {ArrowRight,RefreshCw} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { client, auth } from "@/v3/providers/real";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
export default function AuthPage({
  onAuthenticated,
}: {
  onAuthenticated: () => void;
}) {
  const [setup, setSetup] = useState<boolean | null>(null),
    [password, setPassword] = useState(""),
    [token, setToken] = useState(""),
    [confirmation, setConfirmation] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [invalidField,setInvalidField]=useState<'password'|'token'|'confirmation'|null>(null),
    [checking,setChecking]=useState(true);
  const active = useRef(false),
    passwordField = useRef<HTMLInputElement>(null),
    confirmationField = useRef<HTMLInputElement>(null),
    tokenField = useRef<HTMLInputElement>(null);
  async function readSetup() {
    setError("");setChecking(true);
    try { const value=await client.setupStatus(); if(active.current)setSetup(value.setupAvailable); }
    catch { if(active.current){setSetup(null);setError("暂时无法读取初始化状态，请重新读取。");} }
    finally {if(active.current)setChecking(false);}
  }
  useEffect(() => { active.current=true; void readSetup(); return()=>{active.current=false;}; }, []);
  async function submit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || setup === null) return;
    if (password.length < 12) {
      setInvalidField('password');
      setError("密码至少需要 12 个字符。");
      passwordField.current?.focus();
      return;
    }
    if (setup && token.length < 16) {
      setInvalidField('token');
      setError("请输入有效的初始化令牌。");
      tokenField.current?.focus();
      return;
    }
    if (setup && confirmation !== password) {
      setInvalidField('confirmation');
      setError("两次输入的密码不一致。");
      confirmationField.current?.focus();
      return;
    }
    setBusy(true);
    setError("");
    setInvalidField(null);
    try {
      if (setup) await auth.setup(token, password);
      else await auth.login(password);
      if (active.current) {
        setPassword("");
        setToken("");
        setConfirmation("");
        onAuthenticated();
        location.hash = "#/library";
      }
    } catch (cause) {
      if (active.current)
        setError(
          cause && typeof cause === "object" && "status" in cause && cause.status === 401
            ? setup ? "密码或初始化令牌未被接受，请核对后重试。" : "密码未被接受，请核对后重试。"
            : cause instanceof Error
            ? cause.message
            : "登录未被接受，请核对后重试。",
        );
    } finally {
      if (active.current) setBusy(false);
    }
  }
  return (
    <main className="replica-auth-page">
      <div className="replica-auth-brand"><StoryMark/><div><b>Fluctlight</b><span>A LIFE OF THEIR OWN</span></div></div>
      <Card className="replica-auth-frame">
        <section className="replica-auth-story" aria-label="日常的故事"><div className="replica-eyebrow">WELCOME TO FLUCTLIGHT</div><h1>日常很长，<br/>有你就很好。</h1><p>一个独立生活的世界，<br/>和一些值得慢慢认识的人。</p><WorldArt kind="city"/></section>
        <div className="replica-auth-form">
        <CardHeader>
          <CardTitle><h2>{setup ? "开始你的故事" : "欢迎回来"}</h2></CardTitle><p>{checking?'正在读取账户初始化状态…':setup?'使用部署初始化令牌，设置你的所有者密码。':'使用当前部署的所有者密码，回到你的工作区。'}</p>
        </CardHeader>
        <CardContent>
          <form className="grid gap-5" onSubmit={submit} noValidate>
            {setup && (
              <div className="grid gap-2">
                <Label htmlFor="setup-token">初始化令牌</Label>
                <Input
                  ref={tokenField}
                  id="setup-token"
                  type="password"
                  minLength={16}
                  required
                  autoComplete="off"
                  value={token}
                  onChange={(e) => {setToken(e.target.value);if(invalidField==='token'){setInvalidField(null);setError('');}}}
                  disabled={busy}
                  aria-invalid={invalidField==='token'}
                  aria-describedby={invalidField==='token'?'setup-token-error':undefined}
                />
                {invalidField==='token'&&error&&<p id="setup-token-error" role="alert" data-feedback-kind="error" className="text-xs text-destructive">{error}</p>}
              </div>
            )}
            <div className="grid gap-2">
              <Label htmlFor="owner-password">所有者密码</Label>
              <Input
                ref={passwordField}
                id="owner-password"
                type="password"
                minLength={12}
                required
                placeholder="输入至少 12 个字符的所有者密码"
                autoComplete={setup ? "new-password" : "current-password"}
                value={password}
                onChange={(e) => {setPassword(e.target.value);if(invalidField==='password'){setInvalidField(null);setError('');}}}
                disabled={busy}
                aria-invalid={invalidField==='password'}
                aria-describedby={invalidField==='password'?'owner-password-error':undefined}
              />
              {invalidField==='password'&&error&&<p id="owner-password-error" role="alert" data-feedback-kind="error" className="text-xs text-destructive">{error}</p>}
            </div>
            {setup && (
              <div className="grid gap-2">
                <Label htmlFor="owner-password-confirm">再次输入密码</Label>
                <Input
                  ref={confirmationField}
                  id="owner-password-confirm"
                  type="password"
                  minLength={12}
                  required
                  autoComplete="new-password"
                  value={confirmation}
                  onChange={(e) => {setConfirmation(e.target.value);if(invalidField==='confirmation'){setInvalidField(null);setError('');}}}
                  disabled={busy}
                  aria-invalid={invalidField==='confirmation'}
                  aria-describedby={invalidField==='confirmation'?'owner-confirmation-error':undefined}
                />
                {invalidField==='confirmation'&&error&&<p id="owner-confirmation-error" role="alert" data-feedback-kind="error" className="text-xs text-destructive">{error}</p>}
              </div>
            )}
            {error && !invalidField && (
              <p id="auth-error" role="alert" className="text-destructive">
                {error}
              </p>
            )}
            {setup === null && !checking && <Button type="button" variant="outline" onClick={()=>void readSetup()}><RefreshCw size={14}/>重新读取</Button>}
            <p className="replica-auth-hint">密码至少 12 个字符。{setup?"初始化令牌由当前部署提供，两次密码必须完全一致。":"密码仅用于向当前部署认证。"}</p>
            <Button type="submit" disabled={busy || setup === null}>
              {busy ? "正在连接…" : checking ? "正在读取账户状态…" : setup ? "完成初始化" : "登录"}<ArrowRight size={16}/>
            </Button>
          </form>
        </CardContent>
        </div>
      </Card>
    </main>
  );
}
