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
    [error, setError] = useState("");
  const active = useRef(false),
    passwordField = useRef<HTMLInputElement>(null),
    confirmationField = useRef<HTMLInputElement>(null),
    tokenField = useRef<HTMLInputElement>(null);
  async function readSetup() {
    setError("");
    try { const value=await client.setupStatus(); if(active.current)setSetup(value.setupAvailable); }
    catch { if(active.current){setSetup(null);setError("暂时无法读取初始化状态，请重新读取。");} }
  }
  useEffect(() => { active.current=true; void readSetup(); return()=>{active.current=false;}; }, []);
  async function submit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || setup === null) return;
    if (password.length < 12) {
      setError("密码至少需要 12 个字符。");
      passwordField.current?.focus();
      return;
    }
    if (setup && token.length < 16) {
      setError("请输入有效的初始化令牌。");
      tokenField.current?.focus();
      return;
    }
    if (setup && confirmation !== password) {
      setError("两次输入的密码不一致。");
      confirmationField.current?.focus();
      return;
    }
    setBusy(true);
    setError("");
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
          cause instanceof Error
            ? cause.message
            : "登录未被接受，请核对后重试。",
        );
    } finally {
      if (active.current) setBusy(false);
    }
  }
  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>{setup ? "初始化摇光" : "欢迎回来"}</CardTitle>
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
                  autoComplete="off"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  disabled={busy}
                />
              </div>
            )}
            <div className="grid gap-2">
              <Label htmlFor="owner-password">所有者密码</Label>
              <Input
                ref={passwordField}
                id="owner-password"
                type="password"
                autoComplete={setup ? "new-password" : "current-password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={busy}
                aria-invalid={!!error}
                aria-describedby={error ? "auth-error" : undefined}
              />
            </div>
            {setup && (
              <div className="grid gap-2">
                <Label htmlFor="owner-password-confirm">再次输入密码</Label>
                <Input
                  ref={confirmationField}
                  id="owner-password-confirm"
                  type="password"
                  autoComplete="new-password"
                  value={confirmation}
                  onChange={(e) => setConfirmation(e.target.value)}
                  disabled={busy}
                />
              </div>
            )}
            {error && (
              <p id="auth-error" role="alert" className="text-destructive">
                {error}
              </p>
            )}
            {setup === null && <Button type="button" variant="outline" onClick={()=>void readSetup()}>重新读取</Button>}
            <Button type="submit" disabled={busy || setup === null}>
              {busy ? "正在连接…" : setup ? "完成初始化" : "登录"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}
