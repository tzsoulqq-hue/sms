import { useEffect, useMemo, useState } from 'react';
import { MessageSquareText } from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger, ToastMessage, WorkspaceToolbar, useMutation, useQuery, useQueryClient, useToastMessage } from '@/dashboard/module-kit';
import type { SmsProviderConfig } from '@/proto/byte/v/forge/sms/internal/v1/sms_internal';
import { cancelSmsActivation, deleteSmsProviderConfig, listSmsActivations, listSmsProviderConfigs, saveSmsProviderConfig, smsKeys } from './sms-api';
import { newSmsProviderConfig } from './sms-format';
import { OrdersTab } from './orders-tab';
import { ProviderTab } from './provider-tab';

export function SmsPage() {
  const queryClient = useQueryClient();
  const toast = useToastMessage();
  const [selectedConfigId, setSelectedConfigId] = useState('');
  const configsQuery = useQuery({ queryKey: smsKeys.configs, queryFn: listSmsProviderConfigs });
  const activationsQuery = useQuery({ queryKey: smsKeys.activations, queryFn: listSmsActivations, refetchInterval: 5000 });
  const configs = configsQuery.data?.configs || [];
  const selectedConfig = useMemo(() => configs.find((item) => item.provider_config_id === selectedConfigId) || null, [configs, selectedConfigId]);

  useEffect(() => {
    if (!selectedConfigId && configs[0]?.provider_config_id) setSelectedConfigId(configs[0].provider_config_id);
  }, [configs, selectedConfigId]);

  const saveMutation = useMutation({
    mutationFn: saveSmsProviderConfig,
    onSuccess: async (resp) => {
      if (resp.config?.provider_config_id) setSelectedConfigId(resp.config.provider_config_id);
      await queryClient.invalidateQueries({ queryKey: smsKeys.configs });
      toast.showOK('Provider配置已保存');
    },
    onError: toast.showError
  });
  const deleteMutation = useMutation({
    mutationFn: deleteSmsProviderConfig,
    onSuccess: async () => {
      setSelectedConfigId('');
      await queryClient.invalidateQueries({ queryKey: smsKeys.configs });
      toast.showOK('Provider配置已删除');
    },
    onError: toast.showError
  });
  const cancelMutation = useMutation({
    mutationFn: cancelSmsActivation,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: smsKeys.activations });
      toast.showOK('号码已取消');
    },
    onError: toast.showError
  });

  return (
    <>
      <ToastMessage toast={toast.toast} />
      <section className="workspace singlePaneWorkspace">
        <div className="panel">
          <Tabs defaultValue="providers" className="flex min-h-0 flex-1 flex-col">
            <WorkspaceToolbar
              title={<span className="inline-flex items-center gap-2"><MessageSquareText className="size-4" />SMS</span>}
              meta={`${configs.length}个Provider · ${activationsQuery.data?.activations?.length || 0}个订单`}
              tabs={<TabsList><TabsTrigger value="providers">Provider</TabsTrigger><TabsTrigger value="orders">号码订单</TabsTrigger></TabsList>}
            />
            <TabsContent value="providers" className="mt-0 min-h-0 flex-1">
              <ProviderTab
                configs={configs}
                selected={selectedConfig || (selectedConfigId === 'new' ? newSmsProviderConfig() : null)}
                busy={configsQuery.isLoading}
                saving={saveMutation.isPending}
                deleting={deleteMutation.isPending}
                onSelect={setSelectedConfigId}
                onNew={() => setSelectedConfigId('new')}
                onSave={(config: SmsProviderConfig) => saveMutation.mutate(config)}
                onDelete={(id) => deleteMutation.mutate(id)}
              />
            </TabsContent>
            <TabsContent value="orders" className="mt-0 min-h-0 flex-1">
              <OrdersTab
                activations={activationsQuery.data?.activations || []}
                cancelingId={cancelMutation.variables}
                onCancel={(id) => cancelMutation.mutate(id)}
              />
            </TabsContent>
          </Tabs>
        </div>
      </section>
    </>
  );
}
