function initialData() {
  const services = [
    ['classic','Классический массаж',60,3500,'Массаж всего тела с чередованием мягких и более интенсивных движений.','logo-04'],
    ['relax','Расслабляющий массаж',90,4800,'Плавные движения и спокойный ритм для неспешного отдыха.','logo-06'],
    ['back','Массаж спины',45,2800,'Сеанс с акцентом на поясницу, плечи и шейно-воротниковую зону.','logo-02']
  ].map(([id,name,durationMinutes,priceAmount,description,visualSymbol],index) =>
    ({id,name,durationMinutes,priceAmount,description,visualSymbol,currency:'RUB',sortOrder:index+1,isActive:true}));
  return {version:1,services,bookings:[],settings:{
    workDays:[1,2,3,4,5,6,0],start:'09:00',end:'21:00',step:120,breakMinutes:0,blockedDates:[],vacations:[],
    palette:{green:'#284B35',sand:'#D8C6A6',paper:'#F8F5EE',gold:'#BE9D52'},
    animations:true,revealDuration:700,preloaderDuration:APP_CONFIG.preloaderDuration
  },content:{
    headline:'Искусство\nприкосновения',subtitle:'Индивидуальный массаж для тех, кто хочет\nзамедлиться и почувствовать своё тело.',
    servicesTitle:'Виды массажа',about:'В основе нашего подхода — внимание к ощущениям. Перед сеансом мы обсуждаем, каким зонам уделить больше времени и какой темп и сила прикосновения вам комфортны.',
    heroButton:'Выбрать сеанс',bookingButton:'Забронировать'
  }};
}
