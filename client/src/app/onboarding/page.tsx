"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import {
  Rocket,
  Globe,
  Upload,
  UserPlus,
  ArrowRight,
  ArrowLeft,
  Check,
  Loader2,
  FileText,
} from "lucide-react";
import { graphqlClient } from "@/lib/graphql/client";
import { UPDATE_SETTINGS, UPLOAD_LEXICONS, ADD_ADMIN } from "@/lib/graphql/mutations";
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Alert } from "@/components/ui/Alert";

interface OnboardingState {
  domainAuthority: string;
  adminDid: string;
  lexiconsUploaded: boolean;
}

const STEPS = [
  { id: "welcome", title: "Welcome", icon: Rocket },
  { id: "domain", title: "Domain", icon: Globe },
  { id: "admin", title: "Admin", icon: UserPlus },
  { id: "lexicons", title: "Lexicons", icon: FileText },
  { id: "complete", title: "Complete", icon: Check },
];

export default function OnboardingPage() {
  const router = useRouter();
  const [currentStep, setCurrentStep] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [state, setState] = useState<OnboardingState>({
    domainAuthority: "",
    adminDid: "",
    lexiconsUploaded: false,
  });

  // Update settings mutation
  const updateSettingsMutation = useMutation({
    mutationFn: (vars: { domainAuthority?: string }) =>
      graphqlClient.request(UPDATE_SETTINGS, vars),
    onError: (err: Error) => setError(err.message),
  });

  // Add admin mutation
  const addAdminMutation = useMutation({
    mutationFn: (did: string) => graphqlClient.request(ADD_ADMIN, { did }),
    onError: (err: Error) => setError(err.message),
  });

  // Upload lexicons mutation
  const uploadLexiconsMutation = useMutation({
    mutationFn: (zipBase64: string) =>
      graphqlClient.request(UPLOAD_LEXICONS, { zipBase64 }),
    onSuccess: () => {
      setState((s) => ({ ...s, lexiconsUploaded: true }));
    },
    onError: (err: Error) => setError(err.message),
  });

  const handleNext = async () => {
    setError(null);

    // Validate and save on step transitions
    if (currentStep === 1) {
      // Domain step
      if (!state.domainAuthority.trim()) {
        setError("Please enter a domain authority");
        return;
      }
      try {
        await updateSettingsMutation.mutateAsync({
          domainAuthority: state.domainAuthority.trim(),
        });
      } catch {
        return;
      }
    } else if (currentStep === 2) {
      // Admin step
      if (!state.adminDid.trim()) {
        setError("Please enter an admin DID");
        return;
      }
      if (!state.adminDid.startsWith("did:")) {
        setError("Invalid DID format");
        return;
      }
      try {
        await addAdminMutation.mutateAsync(state.adminDid.trim());
      } catch {
        return;
      }
    }

    setCurrentStep((s) => Math.min(s + 1, STEPS.length - 1));
  };

  const handleBack = () => {
    setError(null);
    setCurrentStep((s) => Math.max(s - 1, 0));
  };

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith(".zip")) {
      setError("Please upload a .zip file");
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      const base64 = (reader.result as string).split(",")[1];
      uploadLexiconsMutation.mutate(base64);
    };
    reader.onerror = () => setError("Failed to read file");
    reader.readAsDataURL(file);
    e.target.value = "";
  };

  const handleComplete = () => {
    router.push("/");
  };

  const isLoading =
    updateSettingsMutation.isPending ||
    addAdminMutation.isPending ||
    uploadLexiconsMutation.isPending;

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center p-6">
      <div className="w-full max-w-2xl">
        {/* Progress Steps */}
        <div className="flex items-center justify-center mb-8">
          {STEPS.map((step, index) => {
            const StepIcon = step.icon;
            const isActive = index === currentStep;
            const isComplete = index < currentStep;

            return (
              <div key={step.id} className="flex items-center">
                <div
                  className={`
                    flex items-center justify-center w-10 h-10 rounded-full
                    transition-colors duration-200
                    ${isActive ? "bg-purple-600 text-white" : ""}
                    ${isComplete ? "bg-green-600 text-white" : ""}
                    ${!isActive && !isComplete ? "bg-gray-800 text-gray-500" : ""}
                  `}
                >
                  {isComplete ? (
                    <Check className="h-5 w-5" />
                  ) : (
                    <StepIcon className="h-5 w-5" />
                  )}
                </div>
                {index < STEPS.length - 1 && (
                  <div
                    className={`w-12 h-0.5 mx-2 ${
                      isComplete ? "bg-green-600" : "bg-gray-800"
                    }`}
                  />
                )}
              </div>
            );
          })}
        </div>

        {/* Step Content */}
        <Card className="border-gray-800">
          {error && (
            <div className="p-4 border-b border-gray-800">
              <Alert variant="error" onClose={() => setError(null)}>
                {error}
              </Alert>
            </div>
          )}

          {/* Welcome Step */}
          {currentStep === 0 && (
            <>
              <CardHeader className="text-center">
                <div className="mx-auto w-16 h-16 bg-purple-600/20 rounded-full flex items-center justify-center mb-4">
                  <Rocket className="h-8 w-8 text-purple-400" />
                </div>
                <CardTitle className="text-2xl">Welcome to Hypergoat</CardTitle>
                <CardDescription className="text-base mt-2">
                  Let's set up your AT Protocol AppView server. This wizard will guide you
                  through the initial configuration.
                </CardDescription>
              </CardHeader>
              <CardContent className="text-center pb-8">
                <div className="space-y-3 text-gray-400 mb-6">
                  <p>You'll configure:</p>
                  <ul className="space-y-2">
                    <li className="flex items-center justify-center gap-2">
                      <Globe className="h-4 w-4 text-purple-400" />
                      Your domain authority
                    </li>
                    <li className="flex items-center justify-center gap-2">
                      <UserPlus className="h-4 w-4 text-purple-400" />
                      An admin account
                    </li>
                    <li className="flex items-center justify-center gap-2">
                      <FileText className="h-4 w-4 text-purple-400" />
                      Your lexicon definitions
                    </li>
                  </ul>
                </div>
              </CardContent>
            </>
          )}

          {/* Domain Step */}
          {currentStep === 1 && (
            <>
              <CardHeader>
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-purple-600/20 rounded-full flex items-center justify-center">
                    <Globe className="h-5 w-5 text-purple-400" />
                  </div>
                  <div>
                    <CardTitle>Domain Authority</CardTitle>
                    <CardDescription>
                      The domain where your AppView will be hosted
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">
                    Domain Authority
                  </label>
                  <Input
                    type="text"
                    placeholder="appview.example.com"
                    value={state.domainAuthority}
                    onChange={(e) =>
                      setState((s) => ({ ...s, domainAuthority: e.target.value }))
                    }
                  />
                  <p className="text-xs text-gray-500 mt-2">
                    This should be the public domain where your AppView is accessible.
                    It's used for OAuth and federation.
                  </p>
                </div>
              </CardContent>
            </>
          )}

          {/* Admin Step */}
          {currentStep === 2 && (
            <>
              <CardHeader>
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-purple-600/20 rounded-full flex items-center justify-center">
                    <UserPlus className="h-5 w-5 text-purple-400" />
                  </div>
                  <div>
                    <CardTitle>Admin Account</CardTitle>
                    <CardDescription>
                      Add the first administrator for your AppView
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">
                    Admin DID
                  </label>
                  <Input
                    type="text"
                    placeholder="did:plc:..."
                    value={state.adminDid}
                    onChange={(e) =>
                      setState((s) => ({ ...s, adminDid: e.target.value }))
                    }
                    className="font-mono"
                  />
                  <p className="text-xs text-gray-500 mt-2">
                    Enter your AT Protocol DID. This account will have full admin access.
                    You can find your DID in your Bluesky profile settings.
                  </p>
                </div>
              </CardContent>
            </>
          )}

          {/* Lexicons Step */}
          {currentStep === 3 && (
            <>
              <CardHeader>
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-purple-600/20 rounded-full flex items-center justify-center">
                    <FileText className="h-5 w-5 text-purple-400" />
                  </div>
                  <div>
                    <CardTitle>Lexicons</CardTitle>
                    <CardDescription>
                      Upload your AT Protocol lexicon definitions
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4">
                {state.lexiconsUploaded ? (
                  <div className="flex items-center gap-3 p-4 bg-green-500/10 border border-green-500/20 rounded-lg">
                    <Check className="h-5 w-5 text-green-400" />
                    <div>
                      <div className="font-medium text-green-400">
                        Lexicons uploaded successfully
                      </div>
                      <div className="text-sm text-gray-400">
                        You can upload more lexicons later from the Lexicons page
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="border-2 border-dashed border-gray-700 rounded-lg p-8 text-center">
                    <Upload className="h-10 w-10 text-gray-500 mx-auto mb-4" />
                    <p className="text-gray-400 mb-4">
                      Upload a ZIP file containing your lexicon JSON files
                    </p>
                    <input
                      type="file"
                      id="lexicon-upload"
                      className="hidden"
                      accept=".zip"
                      onChange={handleFileUpload}
                    />
                    <label htmlFor="lexicon-upload">
                      <Button
                        as="span"
                        disabled={uploadLexiconsMutation.isPending}
                        className="cursor-pointer"
                      >
                        {uploadLexiconsMutation.isPending ? (
                          <>
                            <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                            Uploading...
                          </>
                        ) : (
                          <>
                            <Upload className="h-4 w-4 mr-2" />
                            Choose File
                          </>
                        )}
                      </Button>
                    </label>
                  </div>
                )}
                <p className="text-xs text-gray-500">
                  You can skip this step and upload lexicons later. Your AppView will only
                  index records matching your installed lexicons.
                </p>
              </CardContent>
            </>
          )}

          {/* Complete Step */}
          {currentStep === 4 && (
            <>
              <CardHeader className="text-center">
                <div className="mx-auto w-16 h-16 bg-green-600/20 rounded-full flex items-center justify-center mb-4">
                  <Check className="h-8 w-8 text-green-400" />
                </div>
                <CardTitle className="text-2xl">Setup Complete!</CardTitle>
                <CardDescription className="text-base mt-2">
                  Your Hypergoat AppView is ready to go.
                </CardDescription>
              </CardHeader>
              <CardContent className="text-center pb-8">
                <div className="bg-gray-900 rounded-lg p-4 mb-6 text-left">
                  <h4 className="text-sm font-medium text-gray-300 mb-3">Configuration Summary</h4>
                  <dl className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Domain:</dt>
                      <dd className="font-mono text-white">{state.domainAuthority || "Not set"}</dd>
                    </div>
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Admin:</dt>
                      <dd className="font-mono text-white truncate max-w-[200px]">
                        {state.adminDid || "Not set"}
                      </dd>
                    </div>
                    <div className="flex justify-between">
                      <dt className="text-gray-500">Lexicons:</dt>
                      <dd className="text-white">
                        {state.lexiconsUploaded ? "Uploaded" : "Skipped"}
                      </dd>
                    </div>
                  </dl>
                </div>
                <p className="text-gray-400 text-sm">
                  You can modify these settings anytime from the Settings page.
                </p>
              </CardContent>
            </>
          )}

          {/* Navigation */}
          <div className="flex items-center justify-between p-6 border-t border-gray-800">
            <Button
              variant="secondary"
              onClick={handleBack}
              disabled={currentStep === 0 || isLoading}
            >
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back
            </Button>

            {currentStep === STEPS.length - 1 ? (
              <Button onClick={handleComplete}>
                Go to Dashboard
                <ArrowRight className="h-4 w-4 ml-2" />
              </Button>
            ) : (
              <Button onClick={handleNext} disabled={isLoading}>
                {isLoading ? (
                  <>
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                    Saving...
                  </>
                ) : (
                  <>
                    {currentStep === 3 ? "Finish" : "Next"}
                    <ArrowRight className="h-4 w-4 ml-2" />
                  </>
                )}
              </Button>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
