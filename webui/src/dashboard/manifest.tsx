import { MessageSquareText } from 'lucide-react';
import { DashboardNavSection, type DashboardModuleRegistration } from '@/dashboard/module-kit';
import { SmsPage } from './sms-page';

const registration: DashboardModuleRegistration = {
  manifest: {
    id: 'sms',
    nav: [
      {
        key: 'sms',
        label: 'SMS',
        icon: 'sms',
        section: DashboardNavSection.DASHBOARD_NAV_SECTION_MAIN,
        required_services: ['sms'],
        order: 30
      }
    ]
  },
  icons: {
    sms: <MessageSquareText size={17} />
  },
  views: {
    sms: () => <SmsPage />
  }
};

export default registration;
