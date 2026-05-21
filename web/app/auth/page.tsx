import { Suspense } from "react";
import { AuthForm } from "../../components/AuthForm";

export default function AuthPage() {
  return (
    <Suspense>
      <AuthForm />
    </Suspense>
  );
}
